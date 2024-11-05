package location

import (
	"context"
	"github.com/google/uuid"
	"github.com/yugabyte/pgx/v5"
	"github.com/yugabyte/pgx/v5/pgxpool"
	"log/slog"
	"time"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateLocation(ctx context.Context, location *Location) (*Location, error) {
	var newLocation Location
	err := r.db.QueryRow(ctx,
		`INSERT INTO address (street, city, state_cd, postal_cd, country_cd)
                  VALUES ($1, $2, $3, $4, $5)
               RETURNING id, latitude, longitude`,
		location.Street, location.City, location.State, location.PostalCode, location.Country).
		Scan(&newLocation.AddressId, &newLocation.Latitude, &newLocation.Longitude)
	if err != nil {
		return nil, err
	}

	return &newLocation, nil
}

func (r *Repository) GetLocationById(ctx context.Context, id uuid.UUID) (*Location, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := r.db.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	// enable yb_read_from_follower BEFORE the BEGIN TX (we'll reset it too at the end)
	_, _ = conn.Exec(ctx, "set yb_read_from_followers = true")

	//_, _ = conn.Exec(ctx, "SELECT pg_sleep(30)")

	// now we can BEGIN TRANSACTION READ ONLY
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}

	var value string
	_ = tx.QueryRow(ctx, "select current_setting('yb_read_from_followers')").Scan(&value)
	slog.Debug("Running in Tx", slog.String("yb_read_from_followers", value))

	var location Location
	err = tx.QueryRow(ctx,
		`select loc.id, loc.name, loc.description, adr.id, adr.street, adr.city, adr.state_cd, adr.postal_cd, adr.country_cd, adr.longitude, adr.latitude
               from location loc
          left join address adr
                 on loc.address_id = adr.id
              where loc.id=$1
                and loc.active=true`, id).
		Scan(&location.ID, &location.Name, &location.Description, &location.AddressId, &location.Street, &location.City, &location.State, &location.PostalCode, &location.Country, &location.Longitude, &location.Latitude)

	if err != nil {
		_ = tx.Rollback(ctx)
		_, _ = conn.Exec(ctx, "set yb_read_from_followers = false")
		return nil, err
	}

	_ = tx.Commit(ctx)
	_, _ = conn.Exec(ctx, "set yb_read_from_followers = false")

	return &location, nil
}

// return &Location{
//		ID:          uuid.UUID{},
//		Name:        location.Name,
//		Description: location.Description,
//		AddressId:   uuid.UUID{},
//		Street:      location.Street,
//		City:        location.City,
//		State:       location.State,
//		PostalCode:  location.PostalCode,
//		Country:     location.Country,
//		Longitude:   location.Longitude,
//		Latitude:    location.Latitude,
//	}, nil
