package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dgraph-io/dgo/v250/protos/api"
)

// User statuses. Stored on super.user.status and used to gate login.
const (
	UserStatusInvited   = "invited"   // invite issued, registration not completed
	UserStatusActive    = "active"    // registration completed, may sign in
	UserStatusSuspended = "suspended" // blocked by an admin
)

// User is one person. UID is Dgraph's identifier; KratosID is the identity id
// in Kratos, which is the id the client is handed after authentication.
type User struct {
	UID         string    `json:"uid,omitempty"`
	KratosID    string    `json:"super.user.kratos_id,omitempty"`
	Email       string    `json:"super.user.email,omitempty"`
	DisplayName string    `json:"super.user.display_name,omitempty"`
	NameFirst   string    `json:"super.user.name_first,omitempty"`
	NameLast    string    `json:"super.user.name_last,omitempty"`
	Status      string    `json:"super.user.status,omitempty"`
	Admin       bool      `json:"super.user.admin,omitempty"`
	Locale      string    `json:"super.user.locale,omitempty"`
	CreatedAt   time.Time `json:"super.user.created_at,omitempty"`
	UpdatedAt   time.Time `json:"super.user.updated_at,omitempty"`
	LastSeenAt  time.Time `json:"super.user.last_seen_at,omitempty"`

	// DType must be set for the node to be found by type(User) queries.
	DType []string `json:"dgraph.type,omitempty"`

	// Invite is the invitation this user redeemed, if any.
	Invite *Invite `json:"super.user.invite,omitempty"`
}

// UserInput is the writable subset of a user. UpsertUser ignores zero-valued
// fields, so it can be called with only the fields that changed.
type UserInput struct {
	UID         string
	KratosID    string
	Email       string
	DisplayName string
	NameFirst   string
	NameLast    string
	Status      string
	Admin       *bool
	Locale      string
}

// userByEmailQuery is the lookup half of the upsert guard. `v` is the set of
// matching uids; the mutation only runs when that set is empty.
const userByEmailQuery = `
query q($email: string) {
	existing(func: eq(super.user.email, $email)) {
		v as uid
	}
}`

// UpsertUser creates the user for input.Email, or updates the existing node
// with that email, and returns the stored user.
//
// The email is the key, so calling this for the same address twice updates one
// node rather than creating a second. That is what makes inviting the same
// person again idempotent. KratosID is backfilled on later calls, which is how
// the invite -> registration hand-off records the identity that was created.
func (s *Store) UpsertUser(ctx context.Context, in UserInput) (*User, error) {
	if in.Email == "" {
		return nil, errors.New("db: UpsertUser: email is required")
	}

	// Caller already knows the node.
	if in.UID != "" {
		if err := s.patchUser(ctx, in.UID, in); err != nil {
			return nil, err
		}
		return s.UserByUID(ctx, in.UID)
	}

	// Fast path: the email is already taken, so this is an update.
	existing, err := s.UserByEmail(ctx, in.Email)
	switch {
	case err == nil:
		if err := s.patchUser(ctx, existing.UID, in); err != nil {
			return nil, err
		}
		return s.UserByUID(ctx, existing.UID)
	case !errors.Is(err, ErrNotFound):
		return nil, err
	}

	// Create. The @if guard makes the insert conditional on the email still
	// being unused, so a concurrent invite for the same address resolves to a
	// conflict instead of a second node. Without it @upsert only detects the
	// conflict; it does not prevent the write.
	now := time.Now().UTC()

	node := userPatch(in)
	node["uid"] = "_:user"
	node["dgraph.type"] = TypeUser
	node["super.user.email"] = in.Email
	node["super.user.created_at"] = now.Format(time.RFC3339)
	node["super.user.updated_at"] = now.Format(time.RFC3339)

	if in.Status == "" {
		// A user row only appears because someone was invited or registered;
		// until the flow completes they are merely invited.
		node["super.user.status"] = UserStatusInvited
	}

	payload, err := json.Marshal(node)
	if err != nil {
		return nil, fmt.Errorf("db: UpsertUser: marshal: %w", err)
	}

	req := &api.Request{
		Query: userByEmailQuery,
		Vars:  map[string]string{"$email": in.Email},
		Mutations: []*api.Mutation{{
			SetJson: payload,
			Cond:    "@if(eq(len(v), 0))",
		}},
		CommitNow: true,
	}

	resp, err := s.client.NewTxn().Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("db: UpsertUser: create: %w", err)
	}

	// A uid is returned only when the guard passed, i.e. the node was created.
	if uid, created := resp.Uids["user"]; created {
		return s.UserByUID(ctx, uid)
	}

	// The guard blocked the insert: someone else created the node between the
	// lookup above and this transaction. Re-read and patch that node.
	existing, err = s.UserByEmail(ctx, in.Email)
	if err != nil {
		return nil, err
	}

	if err := s.patchUser(ctx, existing.UID, in); err != nil {
		return nil, err
	}

	return s.UserByUID(ctx, existing.UID)
}

// patchUser writes the non-zero fields of in onto an existing user node and
// stamps updated_at.
func (s *Store) patchUser(ctx context.Context, uid string, in UserInput) error {
	node := userPatch(in)
	node["uid"] = uid
	node["dgraph.type"] = TypeUser
	node["super.user.updated_at"] = time.Now().UTC().Format(time.RFC3339)

	payload, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("db: patchUser: marshal: %w", err)
	}

	mu := &api.Mutation{SetJson: payload, CommitNow: true}
	if _, err := s.client.NewTxn().Mutate(ctx, mu); err != nil {
		return fmt.Errorf("db: patchUser: %w", err)
	}

	return nil
}

// userPatch builds the set of predicates from the non-zero fields of in. Dgraph
// treats absent keys as "leave alone", so a zero field is simply not sent and
// the stored value survives.
func userPatch(in UserInput) map[string]any {
	node := make(map[string]any, 8)

	setIf := func(key, value string) {
		if value != "" {
			node[key] = value
		}
	}

	setIf("super.user.kratos_id", in.KratosID)
	setIf("super.user.display_name", in.DisplayName)
	setIf("super.user.name_first", in.NameFirst)
	setIf("super.user.name_last", in.NameLast)
	setIf("super.user.status", in.Status)
	setIf("super.user.locale", in.Locale)

	if in.Admin != nil {
		node["super.user.admin"] = *in.Admin
	}

	return node
}

// userSelect is the block projected when reading a user back.
//
// The invite edge is traversed so a caller can tell whether an account has been
// linked to the invitation that created it, without a second round trip.
const userSelect = `
	uid
	dgraph.type
	super.user.kratos_id
	super.user.email
	super.user.display_name
	super.user.name_first
	super.user.name_last
	super.user.status
	super.user.admin
	super.user.locale
	super.user.created_at
	super.user.updated_at
	super.user.last_seen_at

	super.user.invite {
		uid
		super.invite.email
		super.invite.kratos_id
		super.invite.status
		super.invite.created_at
		super.invite.expires_at
		super.invite.used_at
	}
`

// UserByEmail returns the user with that email, or ErrNotFound.
func (s *Store) UserByEmail(ctx context.Context, email string) (*User, error) {
	q := fmt.Sprintf(`query q($email: string) {
		u(func: eq(super.user.email, $email)) {%s}
	}`, userSelect)

	return s.queryOneUser(ctx, q, map[string]string{"$email": email})
}

// UserByKratosID returns the user that owns a Kratos identity id, or
// ErrNotFound. This is how a session is mapped back to a stored account.
func (s *Store) UserByKratosID(ctx context.Context, kratosID string) (*User, error) {
	q := fmt.Sprintf(`query q($id: string) {
		u(func: eq(super.user.kratos_id, $id)) {%s}
	}`, userSelect)

	return s.queryOneUser(ctx, q, map[string]string{"$id": kratosID})
}

// UserByUID returns a user by Dgraph uid, or ErrNotFound.
func (s *Store) UserByUID(ctx context.Context, uid string) (*User, error) {
	q := fmt.Sprintf(`query q($uid: string) {
		u(func: uid($uid)) {%s}
	}`, userSelect)

	return s.queryOneUser(ctx, q, map[string]string{"$uid": uid})
}

// queryOneUser runs q and unmarshals the single `u` result, mapping an empty
// result to ErrNotFound.
func (s *Store) queryOneUser(ctx context.Context, q string, vars map[string]string) (*User, error) {
	resp, err := s.client.NewTxn().QueryWithVars(ctx, q, vars)
	if err != nil {
		return nil, fmt.Errorf("db: query user: %w", err)
	}

	var out struct {
		U []User `json:"u"`
	}
	if err := json.Unmarshal(resp.Json, &out); err != nil {
		return nil, fmt.Errorf("db: decode user: %w", err)
	}

	if len(out.U) == 0 {
		return nil, ErrNotFound
	}

	return &out.U[0], nil
}

// SetUserStatus changes a user's status and stamps updated_at.
func (s *Store) SetUserStatus(ctx context.Context, uid, status string) error {
	if status == "" {
		return errors.New("db: SetUserStatus: status is required")
	}

	node := map[string]any{
		"uid":                   uid,
		"super.user.status":     status,
		"super.user.updated_at": time.Now().UTC().Format(time.RFC3339),
	}

	payload, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("db: SetUserStatus: marshal: %w", err)
	}

	mu := &api.Mutation{SetJson: payload, CommitNow: true}
	if _, err := s.client.NewTxn().Mutate(ctx, mu); err != nil {
		return fmt.Errorf("db: SetUserStatus: %w", err)
	}

	return nil
}

// TouchUser records that the user was seen, used on successful login.
func (s *Store) TouchUser(ctx context.Context, uid string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	node := map[string]any{
		"uid":                     uid,
		"super.user.last_seen_at": now,
		"super.user.updated_at":   now,
	}

	payload, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("db: TouchUser: marshal: %w", err)
	}

	mu := &api.Mutation{SetJson: payload, CommitNow: true}
	if _, err := s.client.NewTxn().Mutate(ctx, mu); err != nil {
		return fmt.Errorf("db: TouchUser: %w", err)
	}

	return nil
}

// UsersByStatus returns the users in the given status, ordered by email.
func (s *Store) UsersByStatus(ctx context.Context, status string) ([]User, error) {
	// Dgraph orders with the positional orderasc/orderdesc arguments; the
	// `order: asc(pred)` keyword form is rejected at the root of a query.
	q := fmt.Sprintf(`query q($status: string) {
		u(func: eq(super.user.status, $status), orderasc: super.user.email) {%s}
	}`, userSelect)

	resp, err := s.client.NewTxn().QueryWithVars(ctx, q, map[string]string{"$status": status})
	if err != nil {
		return nil, fmt.Errorf("db: UsersByStatus: %w", err)
	}

	var out struct {
		U []User `json:"u"`
	}
	if err := json.Unmarshal(resp.Json, &out); err != nil {
		return nil, fmt.Errorf("db: UsersByStatus: decode: %w", err)
	}

	return out.U, nil
}

// DeleteUser removes a user node and every triple attached to it. Intended for
// tests and admin tooling, not for the request path.
func (s *Store) DeleteUser(ctx context.Context, uid string) error {
	if uid == "" {
		return errors.New("db: DeleteUser: uid is required")
	}

	return s.deleteNode(ctx, uid)
}
