package yclients

import (
	"context"
	"net/http"
	"net/url"

	bookingcontract "ai-chatbot/contracts/bookingapi"
)

func (c *Client) ListServices(ctx context.Context) ([]bookingcontract.Service, error) {
	var response servicesResponse
	if err := c.do(
		ctx,
		http.MethodGet,
		c.servicesEndpoint(),
		nil,
		nil,
		&response,
		http.StatusOK,
	); err != nil {
		return nil, err
	}
	if !response.Success {
		return nil, ErrUnsuccessfulResponse
	}

	services := make([]bookingcontract.Service, 0, len(response.Data))
	for _, service := range response.Data {
		services = append(services, bookingcontract.Service{
			ID:   string(service.ID),
			Name: service.Title,
		})
	}
	return services, nil
}

func (c *Client) servicesEndpoint() string {
	return "/api/v1/company/" + url.PathEscape(c.config.CompanyID) + "/services"
}
