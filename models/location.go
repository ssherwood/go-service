package models

import "github.com/google/uuid"

type Location struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	AddressId   uuid.UUID `json:"address_id"`
	Street      string    `json:"street"`
	City        string    `json:"city"`
	State       string    `json:"state"`
	PostalCode  string    `json:"postal_code"`
	Country     string    `json:"country"`
	Longitude   float64   `json:"longitude"`
	Latitude    float64   `json:"latitude"`
}
