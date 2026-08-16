package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	bookingcontract "ai-chatbot/contracts/bookingapi"
)

type Workflow struct {
	store       ConversationStore
	interpreter Interpreter
	booking     BookingGateway
	locks       sync.Map
}

func NewWorkflow(store ConversationStore, interpreter Interpreter, booking BookingGateway) *Workflow {
	return &Workflow{store: store, interpreter: interpreter, booking: booking}
}

func (w *Workflow) HandleMessage(ctx context.Context, message InboundMessage) (OutboundMessage, error) {
	conversationID := strings.TrimSpace(message.ConversationID)
	if conversationID == "" || strings.TrimSpace(message.Text) == "" {
		return OutboundMessage{}, fmt.Errorf("message and conversation ID are required")
	}

	lock := w.conversationLock(conversationID)
	lock.Lock()
	defer lock.Unlock()

	//проверяем новый ли диалог
	state, exists, err := w.store.Load(ctx, conversationID)

	if err != nil {
		return OutboundMessage{}, fmt.Errorf("load conversation: %w", err)
	}
	if !exists {
		state, err = w.initializeState(ctx, conversationID)
		if err != nil {
			return OutboundMessage{}, err
		}
	}
	if err := ValidateState(state); err != nil {
		return OutboundMessage{}, err
	}

	if state.Step == StepWaitingForConfirmation && isExplicitConfirmation(message.Text) {
		return w.createBooking(ctx, state)
	}
	if state.Step == StepBookingInProgress || state.Step == StepBooked || state.Step == StepBookingUnknown || state.Step == StepCancelled {
		return RenderReply(state)
	}

	allowed := AllowedActionsFor(state)
	action, err := w.interpreter.Interpret(ctx, InterpretationRequest{Message: message.Text, State: cloneState(state), AllowedActions: allowed})
	if err != nil {
		return OutboundMessage{}, fmt.Errorf("interpret message: %w", err)
	}
	if err := ValidateAction(state, allowed, action); err != nil {
		return OutboundMessage{}, err
	}

	next, effects, err := Transition(state, action)
	if err != nil {
		return OutboundMessage{}, err
	}
	if err := w.applyEffects(ctx, &next, effects); err != nil {
		return OutboundMessage{}, err
	}
	next.UpdatedAt = time.Now().UTC()
	if err := ValidateState(next); err != nil {
		return OutboundMessage{}, err
	}
	if err := w.store.Save(ctx, next); err != nil {
		return OutboundMessage{}, fmt.Errorf("save conversation: %w", err)
	}
	return RenderReply(next)
}

func (w *Workflow) initializeState(ctx context.Context, conversationID string) (ConversationState, error) {
	services, err := w.booking.ListServices(ctx)
	if err != nil {
		return ConversationState{}, fmt.Errorf("list services: %w", err)
	}
	offered := make(map[string]ServiceSnapshot, len(services))
	for _, service := range services {
		id := strings.TrimSpace(service.ID)
		name := strings.TrimSpace(service.Name)
		if id == "" || name == "" {
			return ConversationState{}, fmt.Errorf("list services: invalid service")
		}
		offered[id] = ServiceSnapshot{ID: id, Name: name}
	}
	state := ConversationState{ID: conversationID, Revision: 1, Step: StepWaitingForService, Pending: RequireService, OfferedServices: offered, UpdatedAt: time.Now().UTC()}
	if err := ValidateState(state); err != nil {
		return ConversationState{}, err
	}
	return state, nil
}

func (w *Workflow) applyEffects(ctx context.Context, state *ConversationState, effects []Effect) error {
	for _, effect := range effects {
		switch effect.Type {
		case EffectSearchSlots:
			from := time.Now().UTC()
			slots, err := w.booking.SearchSlots(ctx, bookingcontract.SearchSlotsRequest{ServiceID: effect.ServiceID, From: from, To: from.AddDate(0, 0, 14)})
			if err != nil {
				return fmt.Errorf("search slots: %w", err)
			}
			state.OfferedSlots = make(map[string]SlotSnapshot, len(slots))
			for _, slot := range slots {
				id := strings.TrimSpace(slot.ID)
				if id == "" || slot.ServiceID != effect.ServiceID || slot.StartsAt.IsZero() {
					return fmt.Errorf("search slots: invalid slot")
				}
				state.OfferedSlots[id] = SlotSnapshot{ID: id, ServiceID: slot.ServiceID, StartsAt: slot.StartsAt}
			}
		default:
			return fmt.Errorf("unknown workflow effect %q", effect.Type)
		}
	}
	return nil
}

func (w *Workflow) createBooking(ctx context.Context, state ConversationState) (OutboundMessage, error) {
	operationID, err := newOperationID()
	if err != nil {
		return OutboundMessage{}, err
	}
	inProgress, request, err := BeginBooking(state, operationID)
	if err != nil {
		return OutboundMessage{}, err
	}
	inProgress.UpdatedAt = time.Now().UTC()
	if err := w.store.Save(ctx, inProgress); err != nil {
		return OutboundMessage{}, fmt.Errorf("save booking attempt: %w", err)
	}

	result, createErr := w.booking.CreateBooking(ctx, request)
	if createErr != nil {
		unknown, err := MarkBookingUnknown(inProgress)
		if err != nil {
			return OutboundMessage{}, err
		}
		unknown.UpdatedAt = time.Now().UTC()
		if err := w.store.Save(ctx, unknown); err != nil {
			return OutboundMessage{}, fmt.Errorf("save unknown booking result: %w", err)
		}
		return RenderReply(unknown)
	}

	completed, _ := CompleteBooking(inProgress, result)
	completed.UpdatedAt = time.Now().UTC()
	if err := w.store.Save(ctx, completed); err != nil {
		return OutboundMessage{}, fmt.Errorf("save booking result: %w", err)
	}
	return RenderReply(completed)
}

func RenderReply(state ConversationState) (OutboundMessage, error) {
	if err := ValidateState(state); err != nil {
		return OutboundMessage{}, err
	}

	switch state.Step {
	case StepWaitingForService:
		return OutboundMessage{Kind: OutboundText, Text: renderServices(state.OfferedServices)}, nil
	case StepWaitingForTime:
		return OutboundMessage{Kind: OutboundText, Text: renderSlots(state.OfferedSlots)}, nil
	case StepWaitingForContact:
		if state.Pending == RequireName {
			return OutboundMessage{Kind: OutboundText, Text: "Как вас зовут?"}, nil
		}
		return OutboundMessage{Kind: OutboundText, Text: "Укажите номер телефона."}, nil
	case StepWaitingForConfirmation:
		if state.Booking.Outcome == bookingcontract.BookingRejected {
			return OutboundMessage{Kind: OutboundBookingFailed, Text: "Запись не была создана. Проверьте данные и отправьте «ПОДТВЕРЖДАЮ», чтобы попробовать снова."}, nil
		}
		service := state.OfferedServices[state.ServiceID]
		slot := state.OfferedSlots[state.SelectedSlot]
		text := fmt.Sprintf("Проверьте данные: %s, %s, %s, %s. Для создания записи отправьте «ПОДТВЕРЖДАЮ».", service.Name, slot.StartsAt.Format("02.01.2006 15:04"), state.CustomerName, state.CustomerPhone)
		return OutboundMessage{Kind: OutboundText, Text: text}, nil
	case StepBookingInProgress:
		return OutboundMessage{Kind: OutboundText, Text: "Запись создаётся. Дождитесь результата."}, nil
	case StepBooked:
		return OutboundMessage{Kind: OutboundBookingCreated, Text: fmt.Sprintf("Запись создана. Номер записи: %s.", state.Booking.ExternalID)}, nil
	case StepBookingUnknown:
		return OutboundMessage{Kind: OutboundBookingFailed, Text: "Не удалось однозначно определить результат создания записи. Повторно создавать запись автоматически не будем."}, nil
	case StepCancelled:
		return OutboundMessage{Kind: OutboundText, Text: "Запись отменена."}, nil
	default:
		return OutboundMessage{}, fmt.Errorf("%w: cannot render step %q", ErrInvalidState, state.Step)
	}
}

func renderServices(services map[string]ServiceSnapshot) string {
	if len(services) == 0 {
		return "Сейчас нет доступных услуг."
	}
	ids := make([]string, 0, len(services))
	for id := range services {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	lines := make([]string, 0, len(ids)+1)
	lines = append(lines, "Выберите услугу:")
	for _, id := range ids {
		lines = append(lines, fmt.Sprintf("%s — %s", id, services[id].Name))
	}
	return strings.Join(lines, "\n")
}

func renderSlots(slots map[string]SlotSnapshot) string {
	if len(slots) == 0 {
		return "На ближайшие 14 дней свободных слотов нет."
	}
	ids := make([]string, 0, len(slots))
	for id := range slots {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	lines := make([]string, 0, len(ids)+1)
	lines = append(lines, "Выберите время:")
	for _, id := range ids {
		lines = append(lines, fmt.Sprintf("%s — %s", id, slots[id].StartsAt.Format("02.01.2006 15:04")))
	}
	return strings.Join(lines, "\n")
}

func isExplicitConfirmation(text string) bool {
	return strings.EqualFold(strings.TrimSpace(text), "подтверждаю")
}

func newOperationID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate operation ID: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func (w *Workflow) conversationLock(conversationID string) *sync.Mutex {
	lock, _ := w.locks.LoadOrStore(conversationID, &sync.Mutex{})
	return lock.(*sync.Mutex)
}
