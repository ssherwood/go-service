package location

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/yugabyte/pgx/v5"
	"github.com/yugabyte/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"strconv"
)

type Handler struct {
	db      *pgxpool.Pool
	service *Service
}

func NewHandler(r *mux.Router, service *Service, db *pgxpool.Pool) *Handler {
	handler := &Handler{service: service, db: db}
	r.HandleFunc("/locations", handler.CreateLocation).Methods("POST")
	r.HandleFunc("/locations/{id}", handler.GetLocation).Methods("GET")
	r.HandleFunc("/locations/{id}", handler.UpdateLocation).Methods("PUT")
	r.HandleFunc("/locations/{id}", handler.DeleteLocation).Methods("DELETE")
	return handler
}

func (h *Handler) CreateLocation(w http.ResponseWriter, r *http.Request) {
	var location Location
	if err := json.NewDecoder(r.Body).Decode(&location); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	newLocation, err := h.service.CreateLocation(r.Context(), &location)
	if err != nil {
		// TODO better error handling conditions/types
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(newLocation)
}

func (h *Handler) GetLocation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	currentSpan := trace.SpanFromContext(ctx)
	currentSpan.AddEvent("GetLocation")

	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid location ID", http.StatusBadRequest)
		currentSpan.RecordError(err)
		currentSpan.SetStatus(codes.Error, err.Error())
		return
	}

	location, err := h.service.GetLocationById(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "Location not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		currentSpan.RecordError(err)
		currentSpan.SetStatus(codes.Error, err.Error())
		return
	}

	json.NewEncoder(w).Encode(location)
}

func (h *Handler) UpdateLocation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid location ID", http.StatusBadRequest)
		return
	}

	var location Location
	if err := json.NewDecoder(r.Body).Decode(&location); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	commandTag, err := h.db.Exec(context.Background(),
		"UPDATE locations SET name=$1, address=$2, city=$3, state=$4, zip_code=$5, country=$6 WHERE id=$7",
		location.Name, location.Street, location.City, location.State, location.PostalCode, location.Country, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if commandTag.RowsAffected() == 0 {
		http.Error(w, "Location not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(location)
}

func (h *Handler) DeleteLocation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid location ID", http.StatusBadRequest)
		return
	}

	commandTag, err := h.db.Exec(context.Background(),
		"DELETE FROM locations WHERE id=$1", id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if commandTag.RowsAffected() == 0 {
		http.Error(w, "Location not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
