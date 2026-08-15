package application

import (
	"context"
	"errors"
)

var ErrWorkflowNotImplemented = errors.New("workflow is not implemented")

type Workflow struct {
	store       ConversationStore
	interpreter Interpreter
	booking     BookingGateway
}

func NewWorkflow(store ConversationStore, interpreter Interpreter, booking BookingGateway) *Workflow {
	return &Workflow{
		store:       store,
		interpreter: interpreter,
		booking:     booking,
	}
}

// implement me
func (w *Workflow) HandleMessage(_ context.Context, _ InboundMessage) (OutboundMessage, error) {
	return OutboundMessage{}, ErrWorkflowNotImplemented
}
