// Package db is the Dgraph persistence layer for the super server.
//
// schema.dql declares two node types:
//
//	User    One node per person, keyed by the Kratos identity id and the email
//	        address. Written when an invitation is issued and when the
//	        registration that redeems it completes.
//	Invite  One node per invited email address, created by an admin invitation
//	        (AuthService.InviteUser) and consumed by the registration that
//	        redeems it (AuthService.SubmitAuth).
//
// Every write goes through an upsert guard, so replaying an invitation or a
// registration is idempotent instead of creating a second node. See the notes
// on @upsert in schema.dql for why the guard, and not the directive, is what
// provides the uniqueness guarantee.
//
// ApplySchema runs once on boot. Dgraph's Alter is additive and idempotent, so
// re-applying an unchanged schema is free.
package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	_ "embed"

	dgo "github.com/dgraph-io/dgo/v250"
	"github.com/dgraph-io/dgo/v250/protos/api"
)

// Dgraph type names. A node is only returned by type(X) queries when its
// dgraph.type predicate lists X.
const (
	TypeUser   = "User"
	TypeInvite = "Invite"
)

// ErrNotFound is returned by the read helpers when no node matches the key.
var ErrNotFound = errors.New("db: not found")

// schemaFile is the DQL schema applied by ApplySchema.
//
//go:embed schema.dql
var schemaFile string

// Store is the Dgraph-backed persistence layer.
type Store struct {
	client *dgo.Dgraph
}

// New wraps an already-connected Dgraph client. See config.dgraph.connect.
func New(client *dgo.Dgraph) *Store {
	return &Store{client: client}
}

// ApplySchema installs the User and Invite types and predicates.
//
// Alter is additive and idempotent, so this is safe to call on every boot; it
// never drops or rewrites predicates. Changing a predicate's type or index in
// schema.dql does require care, because Dgraph cannot alter an existing
// predicate in place.
func (s *Store) ApplySchema(ctx context.Context) error {
	if err := s.client.Alter(ctx, &api.Operation{Schema: schemaFile}); err != nil {
		return fmt.Errorf("db: apply schema: %w", err)
	}

	slog.Info("dgraph schema applied")

	return nil
}

// deleteNode removes every triple attached to uid. It is used by the smoke
// test and does not belong in a request path.
func (s *Store) deleteNode(ctx context.Context, uid string) error {
	mu := &api.Mutation{
		DelNquads: fmt.Appendf(nil, "<%s> * * .", uid),
		CommitNow: true,
	}

	if _, err := s.client.NewTxn().Mutate(ctx, mu); err != nil {
		return fmt.Errorf("db: delete node %s: %w", uid, err)
	}

	return nil
}
