package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dgraph-io/dgo/v250/protos/api"
)

// Invite statuses. Stored on super.invite.status.
const (
	InviteStatusPending  = "pending"  // issued, waiting to be redeemed
	InviteStatusRedeemed = "redeemed" // the invited user completed registration
	InviteStatusRevoked  = "revoked"  // cancelled before redemption
	InviteStatusExpired  = "expired"  // expires_at passed before redemption
)

// Invite is an outstanding or historical admin invitation.
//
// An invite is created before the invitee exists in Kratos, so KratosID holds
// the identity Kratos pre-created for them. That is what lets InviteUser hand
// back an identity_id and lets the later registration resolve the same person.
type Invite struct {
	UID       string    `json:"uid,omitempty"`
	Email     string    `json:"super.invite.email,omitempty"`
	KratosID  string    `json:"super.invite.kratos_id,omitempty"`
	CodeHash  string    `json:"super.invite.code_hash,omitempty"`
	Status    string    `json:"super.invite.status,omitempty"`
	Traits    string    `json:"super.invite.traits,omitempty"`
	CreatedAt time.Time `json:"super.invite.created_at,omitempty"`
	ExpiresAt time.Time `json:"super.invite.expires_at,omitempty"`
	UsedAt    time.Time `json:"super.invite.used_at,omitempty"`

	// DType must be set for the node to be found by type(Invite) queries.
	DType []string `json:"dgraph.type,omitempty"`

	// InvitedBy is the admin that issued the invite, when known.
	InvitedBy *User `json:"super.invite.invited_by,omitempty"`

	// User is the account the invite produced, resolved through the reverse
	// edge on super.user.invite.
	User *User `json:"~super.user.invite,omitempty"`
}

// InviteInput is the writable subset of an invite.
type InviteInput struct {
	Email    string
	KratosID string
	CodeHash string
	Traits   string
	// ExpiresAt defaults to InviteTTL from now when zero.
	ExpiresAt time.Time
	// InvitedByUID links the issuing admin, when known.
	InvitedByUID string
}

// InviteTTL is how long an invitation stays redeemable by default.
const InviteTTL = 7 * 24 * time.Hour

// inviteByEmailQuery is the lookup half of the upsert guard. Re-inviting the
// same address refreshes the pending invite instead of adding a second one.
const inviteByEmailQuery = `
query q($email: string) {
	existing(func: eq(super.invite.email, $email)) {
		v as uid
	}
}`

// UpsertInvite records an invitation for input.Email.
//
// Inviting an address that already has an invite refreshes that invite (new
// code, new expiry, status back to pending), so an admin re-inviting someone
// does not accumulate stale rows. The email is the key.
func (s *Store) UpsertInvite(ctx context.Context, in InviteInput) (*Invite, error) {
	if in.Email == "" {
		return nil, errors.New("db: UpsertInvite: email is required")
	}

	expires := in.ExpiresAt
	if expires.IsZero() {
		expires = time.Now().UTC().Add(InviteTTL)
	}

	// An invite that is already there is refreshed rather than duplicated.
	existing, err := s.InviteByEmail(ctx, in.Email)
	switch {
	case err == nil:
		if err := s.patchInvite(ctx, existing.UID, in, expires); err != nil {
			return nil, err
		}
		return s.InviteByUID(ctx, existing.UID)
	case !errors.Is(err, ErrNotFound):
		return nil, err
	}

	now := time.Now().UTC()

	node := map[string]any{
		"uid":                     "_:invite",
		"dgraph.type":             TypeInvite,
		"super.invite.email":      in.Email,
		"super.invite.status":     InviteStatusPending,
		"super.invite.created_at": now.Format(time.RFC3339),
		"super.invite.expires_at": expires.UTC().Format(time.RFC3339),
	}

	setInviteIf(node, in)

	if in.InvitedByUID != "" {
		node["super.invite.invited_by"] = map[string]any{"uid": in.InvitedByUID}
	}

	payload, err := json.Marshal(node)
	if err != nil {
		return nil, fmt.Errorf("db: UpsertInvite: marshal: %w", err)
	}

	req := &api.Request{
		Query: inviteByEmailQuery,
		Vars:  map[string]string{"$email": in.Email},
		Mutations: []*api.Mutation{{
			SetJson: payload,
			Cond:    "@if(eq(len(v), 0))",
		}},
		CommitNow: true,
	}

	resp, err := s.client.NewTxn().Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("db: UpsertInvite: create: %w", err)
	}

	if uid, created := resp.Uids["invite"]; created {
		return s.InviteByUID(ctx, uid)
	}

	// Someone else created it first: refresh theirs.
	existing, err = s.InviteByEmail(ctx, in.Email)
	if err != nil {
		return nil, err
	}

	if err := s.patchInvite(ctx, existing.UID, in, expires); err != nil {
		return nil, err
	}

	return s.InviteByUID(ctx, existing.UID)
}

// patchInvite refreshes an existing invite with a new code and expiry.
func (s *Store) patchInvite(ctx context.Context, uid string, in InviteInput, expires time.Time) error {
	node := map[string]any{
		"uid":                     uid,
		"dgraph.type":             TypeInvite,
		"super.invite.status":     InviteStatusPending,
		"super.invite.expires_at": expires.UTC().Format(time.RFC3339),
	}

	setInviteIf(node, in)

	// A refresh clears the previous redemption.
	if _, err := s.client.NewTxn().Mutate(ctx, &api.Mutation{
		DelNquads: fmt.Appendf(nil, "<%s> <super.invite.used_at> * .", uid),
		CommitNow: true,
	}); err != nil {
		return fmt.Errorf("db: patchInvite: clear used_at: %w", err)
	}

	if in.InvitedByUID != "" {
		node["super.invite.invited_by"] = map[string]any{"uid": in.InvitedByUID}
	}

	payload, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("db: patchInvite: marshal: %w", err)
	}

	mu := &api.Mutation{SetJson: payload, CommitNow: true}
	if _, err := s.client.NewTxn().Mutate(ctx, mu); err != nil {
		return fmt.Errorf("db: patchInvite: %w", err)
	}

	return nil
}

// setInviteIf copies the optional invite fields onto node.
func setInviteIf(node map[string]any, in InviteInput) {
	if in.KratosID != "" {
		node["super.invite.kratos_id"] = in.KratosID
	}
	if in.CodeHash != "" {
		node["super.invite.code_hash"] = in.CodeHash
	}
	if in.Traits != "" {
		node["super.invite.traits"] = in.Traits
	}
}

// inviteSelect is the block projected when reading an invite back.
const inviteSelect = `
	uid
	dgraph.type
	super.invite.email
	super.invite.kratos_id
	super.invite.code_hash
	super.invite.status
	super.invite.traits
	super.invite.created_at
	super.invite.expires_at
	super.invite.used_at

	super.invite.invited_by {
		uid
		super.user.email
		super.user.display_name
	}
`

// InviteByEmail returns the invite for that email, or ErrNotFound.
func (s *Store) InviteByEmail(ctx context.Context, email string) (*Invite, error) {
	q := fmt.Sprintf(`query q($email: string) {
		i(func: eq(super.invite.email, $email)) {%s}
	}`, inviteSelect)

	return s.queryOneInvite(ctx, q, map[string]string{"$email": email})
}

// InviteByUID returns an invite by Dgraph uid, or ErrNotFound.
func (s *Store) InviteByUID(ctx context.Context, uid string) (*Invite, error) {
	q := fmt.Sprintf(`query q($uid: string) {
		i(func: uid($uid)) {%s}
	}`, inviteSelect)

	return s.queryOneInvite(ctx, q, map[string]string{"$uid": uid})
}

// InviteByKratosID returns the invite that pre-created a Kratos identity, or
// ErrNotFound. Used to resolve an invite from the identity in a session.
func (s *Store) InviteByKratosID(ctx context.Context, kratosID string) (*Invite, error) {
	q := fmt.Sprintf(`query q($id: string) {
		i(func: eq(super.invite.kratos_id, $id)) {%s}
	}`, inviteSelect)

	return s.queryOneInvite(ctx, q, map[string]string{"$id": kratosID})
}

// queryOneInvite runs q and unmarshals the single `i` result, mapping an empty
// result to ErrNotFound.
func (s *Store) queryOneInvite(ctx context.Context, q string, vars map[string]string) (*Invite, error) {
	resp, err := s.client.NewTxn().QueryWithVars(ctx, q, vars)
	if err != nil {
		return nil, fmt.Errorf("db: query invite: %w", err)
	}

	var out struct {
		I []Invite `json:"i"`
	}
	if err := json.Unmarshal(resp.Json, &out); err != nil {
		return nil, fmt.Errorf("db: decode invite: %w", err)
	}

	if len(out.I) == 0 {
		return nil, ErrNotFound
	}

	return &out.I[0], nil
}

// RedeemInvite marks the invite for email as redeemed by uid and links the two
// nodes, in one transaction so the pair cannot diverge.
//
// This is the registration half of the invite flow: once Kratos reports the
// identity for the invited address, the invite is closed out and the user node
// points back at it.
func (s *Store) RedeemInvite(ctx context.Context, email, userUID, kratosID string) error {
	if email == "" {
		return errors.New("db: RedeemInvite: email is required")
	}
	if userUID == "" {
		return errors.New("db: RedeemInvite: user uid is required")
	}

	invite, err := s.InviteByEmail(ctx, email)
	if err != nil {
		return err
	}

	now := time.Now().UTC()

	// Both nodes are written together so a failure cannot leave the invite
	// redeemed but the user unlinked.
	pair := []map[string]any{
		{
			"uid":                  invite.UID,
			"super.invite.status":  InviteStatusRedeemed,
			"super.invite.used_at": now.Format(time.RFC3339),
		},
		{
			"uid":               userUID,
			"super.user.invite": map[string]any{"uid": invite.UID},
		},
	}

	if kratosID != "" {
		pair[0]["super.invite.kratos_id"] = kratosID
	}

	payload, err := json.Marshal(pair)
	if err != nil {
		return fmt.Errorf("db: RedeemInvite: marshal: %w", err)
	}

	mu := &api.Mutation{SetJson: payload, CommitNow: true}
	if _, err := s.client.NewTxn().Mutate(ctx, mu); err != nil {
		return fmt.Errorf("db: RedeemInvite: %w", err)
	}

	return nil
}

// PendingInvites returns the invites still awaiting redemption, newest first.
func (s *Store) PendingInvites(ctx context.Context) ([]Invite, error) {
	// Dgraph orders with the positional orderasc/orderdesc arguments; the
	// `order: asc(pred)` keyword form is rejected at the root of a query.
	q := fmt.Sprintf(`query {
		i(func: eq(super.invite.status, %q), orderdesc: super.invite.created_at) {%s}
	}`, InviteStatusPending, inviteSelect)

	resp, err := s.client.NewTxn().Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("db: PendingInvites: %w", err)
	}

	var out struct {
		I []Invite `json:"i"`
	}
	if err := json.Unmarshal(resp.Json, &out); err != nil {
		return nil, fmt.Errorf("db: PendingInvites: decode: %w", err)
	}

	return out.I, nil
}

// InviteExpired reports whether the invite's expires_at has passed.
//
// Dgraph stores no server-side clock comparison for this, so the check is done
// in Go where the caller can decide how to treat a just-expired invite.
func InviteExpired(i *Invite, now time.Time) bool {
	if i == nil || i.ExpiresAt.IsZero() {
		return false
	}

	return now.After(i.ExpiresAt)
}

// DeleteInvite removes an invite node and every triple attached to it. Intended
// for tests and admin tooling, not for the request path.
func (s *Store) DeleteInvite(ctx context.Context, uid string) error {
	if uid == "" {
		return errors.New("db: DeleteInvite: uid is required")
	}

	return s.deleteNode(ctx, uid)
}
