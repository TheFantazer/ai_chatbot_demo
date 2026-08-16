package application

import (
	"errors"
	"fmt"
	"strings"
)

var ErrActionNotAllowed = errors.New("action is not allowed")

type EffectType string

const (
	EffectSearchSlots EffectType = "search_slots"
)

type Effect struct {
	Type      EffectType
	ServiceID string
}

func AllowedActionsFor(state ConversationState) []ActionType {
	switch state.Step {
	case StepWaitingForService:
		actions := []ActionType{ActionAskQuestion, ActionCancelFlow, ActionClarify}
		if len(state.OfferedServices) > 0 {
			actions = append([]ActionType{ActionChooseService}, actions...)
		}
		return actions
	case StepWaitingForTime:
		actions := []ActionType{ActionChangeService, ActionAskQuestion, ActionCancelFlow, ActionClarify}
		if len(state.OfferedSlots) > 0 {
			actions = append([]ActionType{ActionChooseTime}, actions...)
		}
		return actions
	case StepWaitingForContact:
		return []ActionType{ActionProvideContact, ActionChangeService, ActionChangeTime, ActionAskQuestion, ActionCancelFlow, ActionClarify}
	case StepWaitingForConfirmation:
		return []ActionType{ActionChangeService, ActionChangeTime, ActionAskQuestion, ActionCancelFlow, ActionClarify}
	default:
		return nil
	}
}

func ValidateAction(state ConversationState, allowed []ActionType, action ActionEnvelope) error {
	if err := ValidateState(state); err != nil {
		return err
	}
	if action.StateRevision != state.Revision {
		return fmt.Errorf("%w: state revision mismatch", ErrActionNotAllowed)
	}
	if !containsAction(allowed, action.Action) || !containsAction(AllowedActionsFor(state), action.Action) {
		return fmt.Errorf("%w: %q", ErrActionNotAllowed, action.Action)
	}

	switch action.Action {
	case ActionChooseService:
		if err := validateOfferedService(state, action.Arguments.ServiceID); err != nil {
			return err
		}
	case ActionChooseTime:
		slotID := strings.TrimSpace(action.Arguments.SlotID)
		slot, ok := state.OfferedSlots[slotID]
		if slotID == "" || !ok || slot.ServiceID != state.ServiceID {
			return fmt.Errorf("%w: selected slot was not offered for the current service", ErrActionNotAllowed)
		}
	case ActionProvideContact:
		if state.Pending == RequireName && strings.TrimSpace(action.Arguments.Name) == "" {
			return fmt.Errorf("%w: customer name is required", ErrActionNotAllowed)
		}
		if state.Pending == RequirePhone && strings.TrimSpace(action.Arguments.Phone) == "" {
			return fmt.Errorf("%w: customer phone is required", ErrActionNotAllowed)
		}
	case ActionChangeService:
		if strings.TrimSpace(action.Arguments.ServiceID) != "" {
			if err := validateOfferedService(state, action.Arguments.ServiceID); err != nil {
				return err
			}
		}
	}

	return nil
}

func Transition(state ConversationState, action ActionEnvelope) (ConversationState, []Effect, error) {
	allowed := AllowedActionsFor(state)
	if err := ValidateAction(state, allowed, action); err != nil {
		return ConversationState{}, nil, err
	}

	next := cloneState(state)
	next.Revision++
	effects := make([]Effect, 0, 1)

	switch action.Action {
	case ActionChooseService:
		selectService(&next, strings.TrimSpace(action.Arguments.ServiceID))
		effects = append(effects, Effect{Type: EffectSearchSlots, ServiceID: next.ServiceID})
	case ActionChooseTime:
		next.SelectedSlot = strings.TrimSpace(action.Arguments.SlotID)
		next.Step = StepWaitingForContact
		if strings.TrimSpace(next.CustomerName) == "" {
			next.Pending = RequireName
		} else if strings.TrimSpace(next.CustomerPhone) == "" {
			next.Pending = RequirePhone
		} else {
			next.Step = StepWaitingForConfirmation
			next.Pending = RequirementNone
		}
	case ActionProvideContact:
		if name := strings.TrimSpace(action.Arguments.Name); name != "" {
			next.CustomerName = name
		}
		if phone := strings.TrimSpace(action.Arguments.Phone); phone != "" {
			next.CustomerPhone = phone
		}
		if strings.TrimSpace(next.CustomerName) == "" {
			next.Pending = RequireName
		} else if strings.TrimSpace(next.CustomerPhone) == "" {
			next.Pending = RequirePhone
		} else {
			next.Step = StepWaitingForConfirmation
			next.Pending = RequirementNone
		}
	case ActionChangeService:
		serviceID := strings.TrimSpace(action.Arguments.ServiceID)
		if serviceID == "" {
			resetSelectedService(&next)
		} else {
			selectService(&next, serviceID)
			effects = append(effects, Effect{Type: EffectSearchSlots, ServiceID: next.ServiceID})
		}
	case ActionChangeTime:
		next.SelectedSlot = ""
		next.Booking = BookingAttempt{}
		next.Step = StepWaitingForTime
		next.Pending = RequireTime
	case ActionCancelFlow:
		next.Step = StepCancelled
		next.Pending = RequirementNone
	case ActionAskQuestion, ActionClarify:
	}

	if next.Step == StepBooked || next.Step == StepBookingInProgress || next.Step == StepBookingUnknown {
		return ConversationState{}, nil, fmt.Errorf("%w: model transition attempted a protected booking step", ErrActionNotAllowed)
	}

	return next, effects, nil
}

func validateOfferedService(state ConversationState, serviceID string) error {
	serviceID = strings.TrimSpace(serviceID)
	if serviceID == "" {
		return fmt.Errorf("%w: service ID is required", ErrActionNotAllowed)
	}
	if _, ok := state.OfferedServices[serviceID]; !ok {
		return fmt.Errorf("%w: service was not offered", ErrActionNotAllowed)
	}
	return nil
}

func containsAction(actions []ActionType, action ActionType) bool {
	for _, candidate := range actions {
		if candidate == action {
			return true
		}
	}
	return false
}

func selectService(state *ConversationState, serviceID string) {
	state.ServiceID = serviceID
	state.OfferedSlots = nil
	state.SelectedSlot = ""
	state.CustomerName = ""
	state.CustomerPhone = ""
	state.Booking = BookingAttempt{}
	state.Step = StepWaitingForTime
	state.Pending = RequireTime
}

func resetSelectedService(state *ConversationState) {
	state.ServiceID = ""
	state.OfferedSlots = nil
	state.SelectedSlot = ""
	state.CustomerName = ""
	state.CustomerPhone = ""
	state.Booking = BookingAttempt{}
	state.Step = StepWaitingForService
	state.Pending = RequireService
}
