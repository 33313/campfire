package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Tickets and buckets are one table discriminated by kind (SPEC.md §4). The
// only thing a bucket cannot do that a ticket can is belong to a bucket.
const (
	KindTicket = "ticket"
	KindBucket = "bucket"
)

// Rejection reasons (SPEC.md §7.3).
const (
	ReasonCancelled = "cancelled"
	ReasonDuplicate = "duplicate"
)

// BucketIcons is the fixed set (DESIGN.md §4). It is not extensible and not
// uploadable, and nothing reads an icon or derives behaviour from it: it is
// recognition and nothing more (SPEC.md §6.2).
var BucketIcons = []string{"check", "bug", "bookmark", "page", "pen", "wrench", "lens", "flag"}

// DefaultBucketIcon is what a bucket shows when it has not been given one.
const DefaultBucketIcon = "bookmark"

// BucketColours is the fixed set a bucket may be given, on the same terms as
// the icons: buckets only, not extensible, and nothing derives behaviour from
// one. A bucket without a colour is drawn as it always was.
var BucketColours = []string{"amber", "rust", "olive", "steel", "mauve", "sage", "stone"}

func valid(set []string, v string) bool {
	for _, s := range set {
		if s == v {
			return true
		}
	}
	return false
}

func ValidIcon(icon string) bool { return valid(BucketIcons, icon) }

func ValidColour(colour string) bool { return valid(BucketColours, colour) }

// Identifier renders an item's permanent public name, e.g. SOL-42. The prefix
// is the short code of the team that *created* the item, which is not
// necessarily the team that owns it now (SPEC.md §6.3).
func Identifier(shortCode string, number int64) string {
	return shortCode + "-" + strconv.FormatInt(number, 10)
}

// ParseIdentifier splits SOL-42 into its parts. It deliberately says nothing
// about whether the item is a ticket or a bucket: one format means one pattern
// for finding references in text.
func ParseIdentifier(s string) (shortCode string, number int64, ok bool) {
	i := strings.LastIndex(s, "-")
	if i <= 0 || i == len(s)-1 {
		return "", 0, false
	}
	n, err := strconv.ParseInt(s[i+1:], 10, 64)
	if err != nil || n <= 0 {
		return "", 0, false
	}
	return s[:i], n, true
}

type Item struct {
	ID           int64  `json:"id"`
	Kind         string `json:"kind"`
	Identifier   string `json:"identifier"`
	OriginTeamID int64  `json:"originTeamId"`
	Number       int64  `json:"number"`
	TeamID       int64  `json:"teamId"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	ReporterID   int64  `json:"reporterId"`
	AssigneeID   int64  `json:"assigneeId,omitempty"`
	Priority     int64  `json:"priority"`
	Icon         string `json:"icon,omitempty"`
	Color        string `json:"color,omitempty"`
	BucketID     int64  `json:"bucketId,omitempty"`
	StateID      int64  `json:"stateId"`
	PlannedEnd   string `json:"plannedEnd,omitempty"`
	ActualStart  int64  `json:"actualStart,omitempty"`
	ActualEnd    int64  `json:"actualEnd,omitempty"`
	RejectReason string `json:"rejectReason,omitempty"`
	RejectNote   string `json:"rejectNote,omitempty"`
	Rank         string `json:"rank"`
	Version      int64  `json:"version"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`

	// Joined for display. The client renders from these rather than issuing a
	// request per name.
	TeamShortCode    string   `json:"teamShortCode,omitempty"`
	StateName        string   `json:"stateName,omitempty"`
	StateRole        string   `json:"stateRole,omitempty"`
	ReporterName     string   `json:"reporterName,omitempty"`
	AssigneeName     string   `json:"assigneeName,omitempty"`
	BucketIdentifier string   `json:"bucketIdentifier,omitempty"`
	BucketTitle      string   `json:"bucketTitle,omitempty"`
	BucketIcon       string   `json:"bucketIcon,omitempty"`
	BucketColor      string   `json:"bucketColor,omitempty"`
	Labels           []string `json:"labels,omitempty"`
}

const itemColumns = `i.id, i.kind, i.origin_team_id, i.number, i.team_id, i.title, i.description,
	i.reporter_id, i.assignee_id, i.priority, i.icon, i.color, i.bucket_id, i.state_id, i.planned_end,
	i.actual_start, i.actual_end, i.reject_reason, i.reject_note, i.rank, i.version,
	i.created_at, i.updated_at,
	ot.short_code, t.short_code, s.name, s.role, rep.username, asg.username,
	b.number, bt.short_code, b.title, b.icon, b.color`

const itemJoins = `
	FROM items i
	JOIN teams ot ON ot.id = i.origin_team_id
	JOIN teams t ON t.id = i.team_id
	JOIN work_states s ON s.id = i.state_id
	JOIN users rep ON rep.id = i.reporter_id
	LEFT JOIN users asg ON asg.id = i.assignee_id
	LEFT JOIN items b ON b.id = i.bucket_id
	LEFT JOIN teams bt ON bt.id = b.origin_team_id`

func scanItem(row interface{ Scan(...any) error }) (Item, error) {
	var it Item
	var assignee, bucket, actualStart, actualEnd, bucketNumber sql.NullInt64
	var icon, colour, plannedEnd, reason, note sql.NullString
	var originCode, stateRole, assigneeName sql.NullString
	var bucketCode, bucketTitle, bucketIcon, bucketColour sql.NullString
	err := row.Scan(&it.ID, &it.Kind, &it.OriginTeamID, &it.Number, &it.TeamID, &it.Title,
		&it.Description, &it.ReporterID, &assignee, &it.Priority, &icon, &colour, &bucket, &it.StateID,
		&plannedEnd, &actualStart, &actualEnd, &reason, &note, &it.Rank, &it.Version,
		&it.CreatedAt, &it.UpdatedAt,
		&originCode, &it.TeamShortCode, &it.StateName, &stateRole, &it.ReporterName, &assigneeName,
		&bucketNumber, &bucketCode, &bucketTitle, &bucketIcon, &bucketColour)
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	if err != nil {
		return Item{}, err
	}
	it.AssigneeID, it.BucketID = num(assignee), num(bucket)
	it.ActualStart, it.ActualEnd = num(actualStart), num(actualEnd)
	it.Icon, it.Color, it.PlannedEnd = text(icon), text(colour), text(plannedEnd)
	it.RejectReason, it.RejectNote = text(reason), text(note)
	it.StateRole, it.AssigneeName = text(stateRole), text(assigneeName)
	it.Identifier = Identifier(text(originCode), it.Number)
	if bucketCode.Valid && bucketNumber.Valid {
		it.BucketIdentifier = Identifier(bucketCode.String, bucketNumber.Int64)
		it.BucketTitle, it.BucketIcon = text(bucketTitle), text(bucketIcon)
		it.BucketColor = text(bucketColour)
		if it.BucketIcon == "" {
			it.BucketIcon = DefaultBucketIcon
		}
	}
	return it, nil
}

// CreateItem allocates the identifier and the rank and inserts the row.
//
// It must be called inside a write transaction: the counter increment and the
// insert are one unit (SPEC.md §13.2 rule 2). Callers that create several
// items — a split — call it once per item; a batch never reserves a range.
func CreateItem(ctx context.Context, q Q, it Item) (Item, error) {
	number, err := NextItemNumber(ctx, q, it.OriginTeamID)
	if err != nil {
		return Item{}, err
	}
	it.Number = number
	if it.Rank == "" {
		if it.Rank, err = RankAtEnd(ctx, q, it.StateID); err != nil {
			return Item{}, err
		}
	}
	res, err := q.ExecContext(ctx, `
		INSERT INTO items (kind, origin_team_id, number, team_id, title, description,
		                   reporter_id, assignee_id, priority, icon, color, bucket_id, state_id,
		                   planned_end, actual_start, actual_end, reject_reason, reject_note,
		                   rank, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`,
		it.Kind, it.OriginTeamID, it.Number, it.TeamID, it.Title, it.Description,
		it.ReporterID, null(it.AssigneeID), it.Priority, null(it.Icon), null(it.Color), null(it.BucketID),
		it.StateID, null(it.PlannedEnd), null(it.ActualStart), null(it.ActualEnd),
		null(it.RejectReason), null(it.RejectNote), it.Rank, it.CreatedAt, it.UpdatedAt)
	if err != nil {
		return Item{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Item{}, err
	}
	return ItemByID(ctx, q, id)
}

// IdentifierOf resolves an item's public name without loading the item. The
// audit log and the live-update notification both carry it, and both are on
// the write path, so it is one indexed read rather than a full row.
func IdentifierOf(ctx context.Context, q Q, id int64) (string, error) {
	var code string
	var number int64
	err := q.QueryRowContext(ctx, `
		SELECT ot.short_code, i.number FROM items i
		JOIN teams ot ON ot.id = i.origin_team_id WHERE i.id = ?`, id).Scan(&code, &number)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return Identifier(code, number), nil
}

func ItemByID(ctx context.Context, q Q, id int64) (Item, error) {
	return scanItem(q.QueryRowContext(ctx, `SELECT `+itemColumns+itemJoins+` WHERE i.id = ?`, id))
}

// ItemByIdentifier resolves SOL-42. The prefix is matched against the team
// that minted the number, never against current ownership (SPEC.md §6.3).
func ItemByIdentifier(ctx context.Context, q Q, identifier string) (Item, error) {
	code, number, ok := ParseIdentifier(identifier)
	if !ok {
		return Item{}, ErrNotFound
	}
	return scanItem(q.QueryRowContext(ctx, `SELECT `+itemColumns+itemJoins+`
		WHERE ot.short_code_folded = ? AND i.number = ?`, Fold(code), number))
}

// UpdateItem writes the mutable fields with a compare-and-swap on version.
//
// Never read-then-write (SPEC.md §13.2 rule 1): a caller that read the item,
// decided on a change, and lost a race gets ErrConflict rather than silently
// overwriting the change it did not see.
var ErrConflict = errors.New("the item changed while you were editing it")

// The version on the passed item is advanced on success, so a caller making
// two writes in one transaction — a move followed by a rank, say — does not
// hand back a version it has itself superseded.
func UpdateItem(ctx context.Context, q Q, it *Item) error {
	res, err := q.ExecContext(ctx, `
		UPDATE items SET title = ?, description = ?, assignee_id = ?, priority = ?, icon = ?,
		                 color = ?, bucket_id = ?, state_id = ?, planned_end = ?, actual_start = ?,
		                 actual_end = ?, reject_reason = ?, reject_note = ?, rank = ?,
		                 team_id = ?, version = version + 1, updated_at = ?
		WHERE id = ? AND version = ?`,
		it.Title, it.Description, null(it.AssigneeID), it.Priority, null(it.Icon),
		null(it.Color), null(it.BucketID), it.StateID, null(it.PlannedEnd), null(it.ActualStart),
		null(it.ActualEnd), null(it.RejectReason), null(it.RejectNote), it.Rank,
		it.TeamID, it.UpdatedAt, it.ID, it.Version)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrConflict
	}
	it.Version++
	return nil
}

// DeleteItem hard-deletes. This is an instance-administrator operation and is
// not the ordinary disposal route: rejection with reason `cancelled` is
// (SPEC.md §6.4).
func DeleteItem(ctx context.Context, q Q, id int64) error {
	_, err := q.ExecContext(ctx, `DELETE FROM items WHERE id = ?`, id)
	return err
}

// BucketImpact counts what a bucket's deletion would affect, across how many
// teams. A deletion MUST state this before it happens (SPEC.md §6.4).
func BucketImpact(ctx context.Context, q Q, bucketID int64) (items, teams int, err error) {
	err = q.QueryRowContext(ctx,
		`SELECT count(*), count(DISTINCT team_id) FROM items WHERE bucket_id = ?`, bucketID).
		Scan(&items, &teams)
	return
}

// RankAtEnd returns a rank sorting after everything currently in the state.
func RankAtEnd(ctx context.Context, q Q, stateID int64) (string, error) {
	var last sql.NullString
	if err := q.QueryRowContext(ctx,
		`SELECT max(rank) FROM items WHERE state_id = ?`, stateID).Scan(&last); err != nil {
		return "", err
	}
	return RankBetween(text(last), "")
}

// RankBetweenItems computes the rank for a drop between two neighbours, either
// of which may be 0 for "nothing on that side".
//
// With no neighbour at all the drop is past the last card, so it ranks at the
// end. RankBetween("", "") would instead return the column's fixed midpoint,
// which is where the first card in an empty column already sits.
func RankBetweenItems(ctx context.Context, q Q, stateID, afterID, beforeID int64) (string, error) {
	if afterID == 0 && beforeID == 0 {
		return RankAtEnd(ctx, q, stateID)
	}
	rankOf := func(id int64) (string, error) {
		if id == 0 {
			return "", nil
		}
		var r string
		var state int64
		err := q.QueryRowContext(ctx, `SELECT rank, state_id FROM items WHERE id = ?`, id).Scan(&r, &state)
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		if err != nil {
			return "", err
		}
		if state != stateID {
			return "", fmt.Errorf("neighbour %d is not in that column", id)
		}
		return r, nil
	}
	lo, err := rankOf(afterID)
	if err != nil {
		return "", err
	}
	hi, err := rankOf(beforeID)
	if err != nil {
		return "", err
	}
	return RankBetween(lo, hi)
}

// RankAfterExisting computes a rank between afterID and whatever actually sits
// next to it in the column right now.
//
// It is the retry path SPEC.md §9.3 asks for. Two clients looking at the same
// board can name the same pair of neighbours, so the second one computes the
// midpoint the first has already taken and loses on UNIQUE (state_id, rank).
// Measuring against the true successor instead puts it in the gap immediately
// after afterID — a gap nothing occupies, so the retry cannot lose again, and
// still the position the person dropped it in.
func RankAfterExisting(ctx context.Context, q Q, stateID, afterID int64) (string, error) {
	lo := ""
	if afterID != 0 {
		if err := q.QueryRowContext(ctx,
			`SELECT rank FROM items WHERE id = ?`, afterID).Scan(&lo); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return "", ErrNotFound
			}
			return "", err
		}
	}
	var hi sql.NullString
	if err := q.QueryRowContext(ctx,
		`SELECT min(rank) FROM items WHERE state_id = ? AND rank > ?`, stateID, lo).Scan(&hi); err != nil {
		return "", err
	}
	return RankBetween(lo, text(hi))
}

// ItemFilter is the structured filter of SPEC.md §9.2, and nothing more. There
// is no query language: every dimension here is a column or a join, so a saved
// view can be stored as clauses referencing rows by identity (§9.4).
type ItemFilter struct {
	// IDs restricts to a known set. It is not a filter dimension of §9.2: it
	// is how the bucket index's participation scope hands its set to the same
	// query everything else uses.
	IDs          []int64
	Kinds        []string
	TeamIDs      []int64
	StateIDs     []int64
	StateRoles   []string // states holding one of these roles
	ExcludeRoles []string // states holding none of these roles
	LabelIDs     []int64
	AssigneeIDs  []int64
	Unassigned   bool
	ReporterIDs  []int64
	BucketIDs    []int64
	InAnyBucket  bool
	Priorities   []int64
	Text         string
	// Where is a compiled CSQL condition over the aliases i and s, and its
	// arguments; it narrows whatever else the filter says.
	Where     string
	WhereArgs []any
	// EndedSince drops items that reached a terminal state before this time.
	// Items still open carry no actual end and always pass.
	EndedSince int64
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func (f ItemFilter) where() (string, []any) {
	var w []string
	var args []any
	add := func(col string, vals []int64) {
		if len(vals) == 0 {
			return
		}
		w = append(w, col+" IN ("+placeholders(len(vals))+")")
		for _, v := range vals {
			args = append(args, v)
		}
	}
	if len(f.Kinds) > 0 {
		w = append(w, "i.kind IN ("+placeholders(len(f.Kinds))+")")
		for _, k := range f.Kinds {
			args = append(args, k)
		}
	}
	add("i.id", f.IDs)
	add("i.team_id", f.TeamIDs)
	add("i.state_id", f.StateIDs)
	if len(f.StateRoles) > 0 {
		w = append(w, "s.role IN ("+placeholders(len(f.StateRoles))+")")
		for _, r := range f.StateRoles {
			args = append(args, r)
		}
	}
	if len(f.ExcludeRoles) > 0 {
		w = append(w, "(s.role IS NULL OR s.role NOT IN ("+placeholders(len(f.ExcludeRoles))+"))")
		for _, r := range f.ExcludeRoles {
			args = append(args, r)
		}
	}
	// Unassigned is one more value of the assignee dimension: picked beside
	// people, it widens the choice like any other value.
	switch {
	case f.Unassigned && len(f.AssigneeIDs) > 0:
		w = append(w, "(i.assignee_id IS NULL OR i.assignee_id IN ("+placeholders(len(f.AssigneeIDs))+"))")
		for _, v := range f.AssigneeIDs {
			args = append(args, v)
		}
	case f.Unassigned:
		w = append(w, "i.assignee_id IS NULL")
	default:
		add("i.assignee_id", f.AssigneeIDs)
	}
	add("i.reporter_id", f.ReporterIDs)
	add("i.bucket_id", f.BucketIDs)
	if f.InAnyBucket {
		w = append(w, "i.bucket_id IS NOT NULL")
	}
	add("i.priority", f.Priorities)
	if f.EndedSince > 0 {
		w = append(w, "(i.actual_end IS NULL OR i.actual_end >= ?)")
		args = append(args, f.EndedSince)
	}
	for _, id := range f.LabelIDs {
		// Several label clauses intersect: an item must carry all of them.
		w = append(w, "EXISTS (SELECT 1 FROM item_labels il WHERE il.item_id = i.id AND il.label_id = ?)")
		args = append(args, id)
	}
	if q := strings.TrimSpace(f.Text); q != "" {
		// ponytail: LIKE scan over title and description. At the sizes of
		// SPEC.md §13.5 — under a million rows — this is milliseconds and
		// needs no second index to keep in step. Move to FTS5 if a search
		// measurably drags.
		w = append(w, "(i.title LIKE ? ESCAPE '\\' OR i.description LIKE ? ESCAPE '\\')")
		pattern := "%" + escapeLike(q) + "%"
		args = append(args, pattern, pattern)
	}
	if f.Where != "" {
		w = append(w, f.Where)
		args = append(args, f.WhereArgs...)
	}
	if len(w) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(w, " AND "), args
}

func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// ListItems returns a page of items, keyset-paginated (SPEC.md §13.2 rule 9).
//
// Within one state the order is the manual rank of §9.3; across states it is
// newest first, because a mixed list has no single column to be ranked in.
// The returned cursor is fed back as `cursor` to get the next page.
func ListItems(ctx context.Context, q Q, f ItemFilter, cursor string, limit int) (items []Item, next string, err error) {
	clause, args := f.where()
	byRank := len(f.StateIDs) == 1

	order := " ORDER BY i.rank, i.id LIMIT ?"
	if !byRank {
		order = " ORDER BY i.id DESC LIMIT ?"
	}
	if cursor != "" {
		cond := " AND i.rank > ?"
		if !byRank {
			cond = " AND i.id < ?"
		}
		if clause == "" {
			cond = strings.Replace(cond, " AND ", " WHERE ", 1)
		}
		clause += cond
		args = append(args, cursor)
	}
	args = append(args, limit)

	rows, err := q.QueryContext(ctx, `SELECT `+itemColumns+itemJoins+clause+order, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, "", err
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	if len(items) == limit && limit > 0 {
		last := items[len(items)-1]
		if byRank {
			next = last.Rank
		} else {
			next = strconv.FormatInt(last.ID, 10)
		}
	}
	return items, next, nil
}

// CountItems is what a bucket's progress is drawn from: a count of items in a
// terminal state over the total. It is a view, not a field (SPEC.md §6.2).
func CountItems(ctx context.Context, q Q, f ItemFilter) (int, error) {
	clause, args := f.where()
	var n int
	err := q.QueryRowContext(ctx, `SELECT count(*)`+itemJoins+clause, args...).Scan(&n)
	return n, err
}
