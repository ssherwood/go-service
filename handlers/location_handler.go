package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/yugabyte/pgx/v5"
	"github.com/yugabyte/pgx/v5/pgxpool"
	"locationservice/models"
	"log"
	"net/http"
	"strconv"
)

type LocationHandler struct {
	db *pgxpool.Pool
}

func RegisterLocationHandlers(r *mux.Router, db *pgxpool.Pool) {
	handler := &LocationHandler{db: db}
	r.HandleFunc("/locations", handler.CreateLocation).Methods("POST")
	r.HandleFunc("/locations/{id}", handler.GetLocation).Methods("GET")
	r.HandleFunc("/locations/{id}", handler.UpdateLocation).Methods("PUT")
	r.HandleFunc("/locations/{id}", handler.DeleteLocation).Methods("DELETE")
}

func (h *LocationHandler) CreateLocation(w http.ResponseWriter, r *http.Request) {
	var location models.Location
	if err := json.NewDecoder(r.Body).Decode(&location); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := h.db.QueryRow(context.Background(),
		"INSERT INTO locations (name, address, city, state, zip_code, country) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
		location.Name, location.Street, location.City, location.State, location.PostalCode, location.Country).Scan(&location.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(location)
}

func (h *LocationHandler) GetLocation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid location ID", http.StatusBadRequest)
		return
	}

	var location models.Location

	conn, err := h.db.Acquire(context.Background())
	defer conn.Release()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// enable yb_read_from_follower BEFORE the BEGIN TX (we'll reset it too at the end)
	_, _ = conn.Exec(context.Background(), "set yb_read_from_followers = true")

	tx, err := conn.BeginTx(context.Background(), pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var value string
	_ = tx.QueryRow(context.Background(), "select current_setting('yb_read_from_followers')").Scan(&value)
	log.Println("Running in Tx: yb_read_from_followers is", value)

	// begin timer
	err = tx.QueryRow(context.Background(),
		"select loc.id, loc.name, loc.description, adr.id, adr.street, adr.city, adr.state_cd, adr.postal_cd, adr.country_cd, adr.longitude, adr.latitude from location loc left join address adr on loc.address_id = adr.id where loc.id=$1 and loc.active=true;", id).
		Scan(&location.ID, &location.Name, &location.Description, &location.AddressId, &location.Street, &location.City, &location.State, &location.PostalCode, &location.Country, &location.Longitude, &location.Latitude)
	// end timer?

	if err != nil {
		_ = tx.Rollback(context.Background())
		_, _ = conn.Exec(context.Background(), "set yb_read_from_followers = false")

		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "Location not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	_ = tx.Commit(context.Background())
	_, _ = conn.Exec(context.Background(), "set yb_read_from_followers = false")

	json.NewEncoder(w).Encode(location)
}

func (h *LocationHandler) UpdateLocation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid location ID", http.StatusBadRequest)
		return
	}

	var location models.Location
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

func (h *LocationHandler) DeleteLocation(w http.ResponseWriter, r *http.Request) {
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
