package vk

import "encoding/json"

type apiResponse[T any] struct {
	Response T         `json:"response"`
	Error    *apiError `json:"error"`
}

type apiError struct {
	Code    int    `json:"error_code"`
	Message string `json:"error_msg"`
}

type longPollServerDTO struct {
	Key    string `json:"key"`
	Server string `json:"server"`
	TS     string `json:"ts"`
}

type longPollResponse struct {
	TS      string     `json:"ts"`
	Updates []eventDTO `json:"updates"`
	Failed  int        `json:"failed"`
}

type eventDTO struct {
	Type    string        `json:"type"`
	Object  messageObject `json:"object"`
	GroupID int64         `json:"group_id"`
	EventID string        `json:"event_id"`
	Version string        `json:"v"`
}

type messageObject struct {
	Message messageDTO `json:"message"`
}

type messageDTO struct {
	ID                    int64  `json:"id"`
	ConversationMessageID int64  `json:"conversation_message_id"`
	FromID                int64  `json:"from_id"`
	PeerID                int64  `json:"peer_id"`
	Text                  string `json:"text"`
	Out                   int    `json:"out"`
}

type sendMessageResponse = apiResponse[json.Number]
