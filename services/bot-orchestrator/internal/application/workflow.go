package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
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
	location    *time.Location
	locks       sync.Map
}

func NewWorkflow(store ConversationStore, interpreter Interpreter, booking BookingGateway, location *time.Location) *Workflow {
	if location == nil {
		location = time.UTC
	}
	return &Workflow{store: store, interpreter: interpreter, booking: booking, location: location}
}

func (w *Workflow) HandleMessage(ctx context.Context, message InboundMessage) (OutboundMessage, error) {
	conversationID := strings.TrimSpace(message.ConversationID)
	if conversationID == "" || strings.TrimSpace(message.Text) == "" {
		return OutboundMessage{}, fmt.Errorf("message and conversation ID are required")
	}

	lock := w.conversationLock(conversationID)
	lock.Lock()
	defer lock.Unlock()

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
	if state.Step == StepBookingInProgress || state.Step == StepBookingUnknown {
		return RenderReply(state)
	}

	allowed := AllowedActionsFor(state)
	action := ActionEnvelope{}
	if state.Step == StepWaitingForContact && state.Pending == RequirePhone {
		if phone := ExtractCustomerPhone(message.Text); phone != "" {
			action = ActionEnvelope{Action: ActionProvideContact, Arguments: ActionArguments{Phone: phone}, StateRevision: state.Revision}
		}
	}
	if action.Action == "" {
		action, err = w.interpreter.Interpret(ctx, InterpretationRequest{Message: message.Text, State: cloneState(state), AllowedActions: allowed})
		if err != nil {
			return RenderFallbackReply(state)
		}
	}
	if err := ValidateAction(state, allowed, action); err != nil {
		return RenderFallbackReply(state)
	}
	if err := ValidateMessageBinding(state, message.Text, action); err != nil {
		return RenderFallbackReply(state)
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
	return RenderActionReply(next, action)
}

func RenderFallbackReply(state ConversationState) (OutboundMessage, error) {
	if state.Step == StepBooked {
		return OutboundMessage{Kind: OutboundText, Text: "Эта запись уже оформлена. Хотите записаться ещё раз? Напишите желаемую услугу — начнём новую запись."}, nil
	}
	if state.Step == StepCancelled {
		return OutboundMessage{Kind: OutboundText, Text: "Текущая запись отменена. Если хотите начать новую, напишите, на какую услугу вас записать."}, nil
	}
	return RenderReply(state)
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
		case EffectPrepareDates:
			state.OfferedDates = make(map[string]DateSnapshot, 14)
			today := beginningOfBusinessDay(time.Now().In(w.location))
			for offset := 0; offset < 14; offset++ {
				date := today.AddDate(0, 0, offset).Format(time.DateOnly)
				state.OfferedDates[date] = DateSnapshot{Date: date}
			}
		case EffectLoadStaff:
			staff, err := w.booking.ListStaff(ctx, bookingcontract.ListStaffRequest{ServiceID: effect.ServiceID})
			if err != nil {
				return fmt.Errorf("list staff: %w", err)
			}
			state.OfferedStaff = make(map[string]StaffSnapshot, len(staff))
			for _, item := range staff {
				id := strings.TrimSpace(item.ID)
				name := strings.TrimSpace(item.Name)
				if id == "" || name == "" {
					return errors.New("list staff: invalid staff member")
				}
				state.OfferedStaff[id] = StaffSnapshot{ID: id, Name: name, Specialization: strings.TrimSpace(item.Specialization)}
			}
		case EffectSearchSlots:
			slots, err := w.booking.SearchSlots(ctx, bookingcontract.SearchSlotsRequest{ServiceID: effect.ServiceID, StaffID: effect.StaffID, Date: effect.Date})
			if err != nil {
				return fmt.Errorf("search slots: %w", err)
			}
			state.OfferedSlots = make(map[string]SlotSnapshot, len(slots))
			for _, slot := range slots {
				id := strings.TrimSpace(slot.ID)
				if id == "" || slot.ServiceID != effect.ServiceID || slot.StaffID != effect.StaffID || slot.StartsAt.IsZero() {
					return fmt.Errorf("search slots: invalid slot")
				}
				state.OfferedSlots[id] = SlotSnapshot{ID: id, ServiceID: slot.ServiceID, StaffID: slot.StaffID, Date: effect.Date, StartsAt: slot.StartsAt.In(w.location)}
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
	case StepWaitingForDate:
		service := state.OfferedServices[state.ServiceID]
		return OutboundMessage{Kind: OutboundText, Text: fmt.Sprintf("Отлично, записываемся на «%s». На какой день вам было бы удобно?", service.Name)}, nil
	case StepWaitingForStaff:
		return OutboundMessage{Kind: OutboundText, Text: renderStaff(state)}, nil
	case StepWaitingForTime:
		return OutboundMessage{Kind: OutboundText, Text: renderSlots(state)}, nil
	case StepWaitingForContact:
		if state.Pending == RequireName {
			slot := state.OfferedSlots[state.SelectedSlot]
			staff := state.OfferedStaff[state.StaffID]
			return OutboundMessage{Kind: OutboundText, Text: fmt.Sprintf("Отлично, %s в %s свободно. Выбранный специалист — %s. Как я могу к вам обращаться?", formatHumanDate(slot.StartsAt), slot.StartsAt.Format("15:04"), staff.Name)}, nil
		}
		return OutboundMessage{Kind: OutboundText, Text: fmt.Sprintf("Приятно познакомиться, %s! Оставьте, пожалуйста, номер телефона для записи.", state.CustomerName)}, nil
	case StepWaitingForConfirmation:
		if state.Booking.Outcome == bookingcontract.BookingRejected {
			return OutboundMessage{Kind: OutboundBookingFailed, Text: "Запись не была создана. Проверьте данные и отправьте «ПОДТВЕРЖДАЮ», чтобы попробовать снова."}, nil
		}
		service := state.OfferedServices[state.ServiceID]
		staff := state.OfferedStaff[state.StaffID]
		slot := state.OfferedSlots[state.SelectedSlot]
		text := fmt.Sprintf("Почти готово! Проверьте запись:\n• Услуга: %s\n• Специалист: %s\n• Когда: %s в %s\n• Имя: %s\n• Телефон: %s\n\nЕсли всё верно, отправьте «ПОДТВЕРЖДАЮ» — только после этого я создам запись.", service.Name, staff.Name, formatHumanDate(slot.StartsAt), slot.StartsAt.Format("15:04"), state.CustomerName, state.CustomerPhone)
		return OutboundMessage{Kind: OutboundText, Text: text}, nil
	case StepBookingInProgress:
		return OutboundMessage{Kind: OutboundText, Text: "Запись создаётся. Дождитесь результата."}, nil
	case StepBooked:
		return OutboundMessage{Kind: OutboundBookingCreated, Text: fmt.Sprintf("Готово! Запись подтверждена в YCLIENTS. Номер записи: %s. Если захотите записаться ещё раз, просто напишите мне.", state.Booking.ExternalID)}, nil
	case StepBookingUnknown:
		return OutboundMessage{Kind: OutboundBookingFailed, Text: "Не удалось однозначно определить результат создания записи. Повторно создавать запись автоматически не будем."}, nil
	case StepCancelled:
		return OutboundMessage{Kind: OutboundText, Text: "Хорошо, текущую запись отменили. Если захотите начать заново, просто напишите, что хотите записаться."}, nil
	default:
		return OutboundMessage{}, fmt.Errorf("%w: cannot render step %q", ErrInvalidState, state.Step)
	}
}

func RenderActionReply(state ConversationState, action ActionEnvelope) (OutboundMessage, error) {
	if action.Action == ActionClarify {
		return renderClarification(state, action.Arguments.Topic)
	}
	if action.Action == ActionAskQuestion {
		if state.Step == StepBooked {
			return OutboundMessage{Kind: OutboundText, Text: "Пожалуйста! Если хотите записаться ещё раз, просто скажите — начнём новую запись."}, nil
		}
		return RenderReply(state)
	}
	return RenderReply(state)
}

func renderClarification(state ConversationState, topic string) (OutboundMessage, error) {
	switch state.Step {
	case StepWaitingForService:
		if topic == "unsupported_service" {
			return OutboundMessage{Kind: OutboundText, Text: "Похоже, такой услуги у нас сейчас нет. Но я могу помочь с одной из доступных:\n" + renderServiceNames(state.OfferedServices)}, nil
		}
		return OutboundMessage{Kind: OutboundText, Text: renderServices(state.OfferedServices)}, nil
	case StepWaitingForDate:
		if topic == "out_of_range_date" {
			return OutboundMessage{Kind: OutboundText, Text: "На такую далёкую дату запись пока недоступна — расписание открыто на ближайшие две недели. Какой день в этом диапазоне вам подойдёт?"}, nil
		}
	case StepWaitingForStaff:
		if topic == "unsupported_staff" {
			return OutboundMessage{Kind: OutboundText, Text: "Этого специалиста сейчас нет среди доступных для выбранной услуги. Давайте выберем из списка:\n" + renderStaffNames(state.OfferedStaff)}, nil
		}
	case StepWaitingForTime:
		if topic == "unavailable_time" {
			return OutboundMessage{Kind: OutboundText, Text: "На это время специалист уже занят. Вот актуальные свободные варианты:\n" + renderTimeValues(state.OfferedSlots)}, nil
		}
	case StepBooked:
		return OutboundMessage{Kind: OutboundText, Text: "Если хотите сделать ещё одну запись, просто напишите «хочу записаться» или сразу назовите услугу."}, nil
	}
	return RenderReply(state)
}

func renderServices(services map[string]ServiceSnapshot) string {
	if len(services) == 0 {
		return "Сейчас нет доступных услуг."
	}
	return "Конечно, помогу записаться! Какую услугу вы хотите? Сейчас доступны:\n" + renderServiceNames(services)
}

func renderServiceNames(services map[string]ServiceSnapshot) string {
	names := make([]string, 0, len(services))
	for _, service := range services {
		names = append(names, service.Name)
	}
	sort.Strings(names)
	for index := range names {
		names[index] = "• " + names[index]
	}
	return strings.Join(names, "\n")
}

func renderStaff(state ConversationState) string {
	if len(state.OfferedStaff) == 0 {
		return "На выбранный день подходящих специалистов не нашлось. Давайте попробуем другую дату?"
	}
	return fmt.Sprintf("На %s доступны эти специалисты:\n%s\n\nК кому вас записать?", formatDateOnly(state.SelectedDate), renderStaffNames(state.OfferedStaff))
}

func renderStaffNames(staff map[string]StaffSnapshot) string {
	values := make([]string, 0, len(staff))
	for _, item := range staff {
		value := item.Name
		if item.Specialization != "" {
			value += " — " + item.Specialization
		}
		values = append(values, "• "+value)
	}
	sort.Strings(values)
	return strings.Join(values, "\n")
}

func renderSlots(state ConversationState) string {
	staff := state.OfferedStaff[state.StaffID]
	if len(state.OfferedSlots) == 0 {
		return fmt.Sprintf("На %s у выбранного специалиста свободного времени уже нет. Выберем другого специалиста или другую дату?", formatDateOnly(state.SelectedDate))
	}
	return fmt.Sprintf("%s — свободное время на %s:\n%s\n\nКакое время вам подходит?", staff.Name, formatDateOnly(state.SelectedDate), renderTimeValues(state.OfferedSlots))
}

func renderTimeValues(slots map[string]SlotSnapshot) string {
	values := make([]SlotSnapshot, 0, len(slots))
	for _, slot := range slots {
		values = append(values, slot)
	}
	sort.Slice(values, func(i int, j int) bool {
		if values[i].StartsAt.Equal(values[j].StartsAt) {
			return values[i].ID < values[j].ID
		}
		return values[i].StartsAt.Before(values[j].StartsAt)
	})
	times := make([]string, 0, len(values))
	for _, slot := range values {
		times = append(times, slot.StartsAt.Format("15:04"))
	}
	return strings.Join(times, ", ")
}

func formatDateOnly(value string) string {
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return value
	}
	months := [...]string{"января", "февраля", "марта", "апреля", "мая", "июня", "июля", "августа", "сентября", "октября", "ноября", "декабря"}
	return fmt.Sprintf("%d %s", parsed.Day(), months[parsed.Month()-1])
}

func formatHumanDate(value time.Time) string {
	today := beginningOfBusinessDay(time.Now().In(value.Location()))
	selected := beginningOfBusinessDay(value)
	switch selected.Sub(today) / (24 * time.Hour) {
	case 0:
		return "сегодня"
	case 1:
		return "завтра"
	default:
		return value.Format("02.01.2006")
	}
}

func beginningOfBusinessDay(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, value.Location())
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
