package storage

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	driverName = "sqlite3"
)

// Store wraps the SQLite database and exposes higher-level helpers.
type Store struct {
	db   *sql.DB
	path string
}

// Account represents a customer account.
type Account struct {
	ID            int64
	Name          string
	Phone         string
	Address       string
	Email         string
	DecisionMaker string
	Creator       string
	CreatedAt     time.Time
}

// Note represents a free-form note tied to an optional account.
type Note struct {
	ID          int64
	Content     string
	AccountID   sql.NullInt64
	Creator     string
	CreatedAt   time.Time
	AccountName sql.NullString
}

// Event holds scheduled interactions tied to an optional account.
type Event struct {
	ID          int64
	Title       string
	Details     string
	EventTime   time.Time
	AccountID   sql.NullInt64
	Creator     string
	CreatedAt   time.Time
	AccountName sql.NullString
}

// Activity is a combined stream of user actions for dashboards.
type Activity struct {
	ID        int64
	Type      string
	Title     string
	Details   string
	CreatedAt time.Time
}

// ImportResult summarizes a CSV import operation.
type ImportResult struct {
	Created int
	Skipped int
	Errors  []string
}

var (
	// ErrAccountExists indicates a duplicate account name.
	ErrAccountExists = errors.New("account already exists")
	// ErrNotFound indicates the requested record does not exist.
	ErrNotFound = errors.New("record not found")
)

// Open bootstraps the SQLite store at the default path.
func Open(ctx context.Context) (*Store, error) {
	path, err := resolveDBPath()
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(driverName, path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	store := &Store{db: db, path: path}
	if err := store.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

// Close releases DB resources.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func resolveDBPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = os.Getenv("HOME")
		if base == "" {
			return "", fmt.Errorf("cannot resolve data dir: %w", err)
		}
	}
	dir := filepath.Join(base, "crmterm")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create db dir: %w", err)
	}
	return filepath.Join(dir, "crmterm.db"), nil
}

func (s *Store) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS accounts (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL UNIQUE,
            phone TEXT,
            address TEXT,
            email TEXT,
            decision_maker TEXT,
            creator TEXT NOT NULL,
            created_at TEXT NOT NULL
        );`,
		`CREATE TABLE IF NOT EXISTS notes (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            content TEXT NOT NULL,
            account_id INTEGER,
            creator TEXT NOT NULL,
            created_at TEXT NOT NULL,
            FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE SET NULL
        );`,
		`CREATE TABLE IF NOT EXISTS events (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            title TEXT NOT NULL,
            details TEXT,
            event_time TEXT NOT NULL,
            account_id INTEGER,
            creator TEXT NOT NULL,
            created_at TEXT NOT NULL,
            FOREIGN KEY(account_id) REFERENCES accounts(id) ON DELETE SET NULL
        );`,
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migrations: %w", err)
	}
	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			tx.Rollback()
			return fmt.Errorf("migrate: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}
	return nil
}

// ListAccounts loads all accounts ordered alphabetically.
func (s *Store) ListAccounts(ctx context.Context) ([]Account, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, phone, address, email, decision_maker, creator, created_at FROM accounts ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("query accounts: %w", err)
	}
	defer rows.Close()

	var accounts []Account
	for rows.Next() {
		account, err := scanAccount(rows)
		if err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("accounts rows: %w", err)
	}
	return accounts, nil
}

// SearchAccounts performs a case-insensitive substring search on account names.
func (s *Store) SearchAccounts(ctx context.Context, term string) ([]Account, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return s.ListAccounts(ctx)
	}
	like := fmt.Sprintf("%%%s%%", strings.ToLower(term))
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, phone, address, email, decision_maker, creator, created_at FROM accounts WHERE lower(name) LIKE ? ORDER BY name COLLATE NOCASE`, like)
	if err != nil {
		return nil, fmt.Errorf("search accounts: %w", err)
	}
	defer rows.Close()

	var accounts []Account
	for rows.Next() {
		account, err := scanAccount(rows)
		if err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return accounts, nil
}

// CreateAccount inserts a new account enforcing uniqueness.
func (s *Store) CreateAccount(ctx context.Context, a *Account) error {
	if strings.TrimSpace(a.Name) == "" {
		return fmt.Errorf("account name required")
	}
	now := time.Now().UTC()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO accounts (name, phone, address, email, decision_maker, creator, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(a.Name), nullString(a.Phone), nullString(a.Address), nullString(a.Email), nullString(a.DecisionMaker), a.Creator, a.CreatedAt.UTC().Format(time.RFC3339))
	if err != nil {
		if isUniqueConstraint(err) {
			return ErrAccountExists
		}
		return fmt.Errorf("insert account: %w", err)
	}
	id, err := res.LastInsertId()
	if err == nil {
		a.ID = id
	}
	return nil
}

// CreateNote persists a new note.
func (s *Store) CreateNote(ctx context.Context, n *Note) error {
	if strings.TrimSpace(n.Content) == "" {
		return fmt.Errorf("note content required")
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO notes (content, account_id, creator, created_at) VALUES (?, ?, ?, ?)`,
		n.Content, nullInt64(n.AccountID), n.Creator, n.CreatedAt.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert note: %w", err)
	}
	return nil
}

// CreateEvent persists a new event.
func (s *Store) CreateEvent(ctx context.Context, e *Event) error {
	if strings.TrimSpace(e.Title) == "" {
		return fmt.Errorf("event title required")
	}
	if e.EventTime.IsZero() {
		e.EventTime = time.Now().UTC()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO events (title, details, event_time, account_id, creator, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		e.Title, nullString(e.Details), e.EventTime.UTC().Format(time.RFC3339), nullInt64(e.AccountID), e.Creator, e.CreatedAt.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

// AccountByName retrieves an account by case-insensitive name.
func (s *Store) AccountByName(ctx context.Context, name string) (*Account, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, phone, address, email, decision_maker, creator, created_at FROM accounts WHERE lower(name) = lower(?)`, strings.TrimSpace(name))
	account, err := scanAccount(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get account: %w", err)
	}
	return &account, nil
}

// AccountByID retrieves an account by its identifier.
func (s *Store) AccountByID(ctx context.Context, id int64) (*Account, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, phone, address, email, decision_maker, creator, created_at FROM accounts WHERE id = ?`, id)
	account, err := scanAccount(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get account: %w", err)
	}
	return &account, nil
}

// UpdateAccount persists changes to an existing account.
func (s *Store) UpdateAccount(ctx context.Context, a *Account) error {
	if a == nil {
		return fmt.Errorf("nil account")
	}
	if strings.TrimSpace(a.Name) == "" {
		return fmt.Errorf("account name required")
	}
	_, err := s.db.ExecContext(ctx, `UPDATE accounts SET name = ?, phone = ?, address = ?, email = ?, decision_maker = ? WHERE id = ?`,
		strings.TrimSpace(a.Name), nullString(a.Phone), nullString(a.Address), nullString(a.Email), nullString(a.DecisionMaker), a.ID)
	if err != nil {
		if isUniqueConstraint(err) {
			return ErrAccountExists
		}
		return fmt.Errorf("update account: %w", err)
	}
	return nil
}

// ListEvents fetches events sorted by event_time ascending.
func (s *Store) ListEvents(ctx context.Context) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT e.id, e.title, e.details, e.event_time, e.account_id, e.creator, e.created_at, a.name
        FROM events e
        LEFT JOIN accounts a ON a.id = e.account_id
        ORDER BY e.event_time ASC`)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var eventTime, created string
		var accountID sql.NullInt64
		var accountName sql.NullString
		if err := rows.Scan(&e.ID, &e.Title, &e.Details, &eventTime, &accountID, &e.Creator, &created, &accountName); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		if t, err := time.Parse(time.RFC3339, eventTime); err == nil {
			e.EventTime = t
		}
		if t, err := time.Parse(time.RFC3339, created); err == nil {
			e.CreatedAt = t
		}
		e.AccountID = accountID
		e.AccountName = accountName
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

// ListActivities returns a combined stream sorted newest first.
func (s *Store) ListActivities(ctx context.Context, limit int) ([]Activity, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `SELECT type, id, title, details, created_at FROM (
            SELECT 'account' AS type, id, name AS title, phone AS details, created_at FROM accounts
            UNION ALL
            SELECT 'note' AS type, id, substr(content, 1, 80) AS title, '' AS details, created_at FROM notes
            UNION ALL
            SELECT 'event' AS type, id, title, substr(details, 1, 80) AS details, created_at FROM events
        ) ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query activities: %w", err)
	}
	defer rows.Close()

	var activities []Activity
	for rows.Next() {
		var a Activity
		var details sql.NullString
		var created string
		if err := rows.Scan(&a.Type, &a.ID, &a.Title, &details, &created); err != nil {
			return nil, fmt.Errorf("scan activity: %w", err)
		}
		a.Details = nullStringToString(details)
		if t, err := time.Parse(time.RFC3339, created); err == nil {
			a.CreatedAt = t
		}
		activities = append(activities, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return activities, nil
}

// ListAccountActivity returns activity related to a specific account.
func (s *Store) ListAccountActivity(ctx context.Context, accountID int64, limit int) ([]Activity, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `SELECT type, id, title, details, created_at FROM (
            SELECT 'account' AS type, id, name AS title, phone AS details, created_at FROM accounts WHERE id = ?
            UNION ALL
            SELECT 'note' AS type, id, substr(content, 1, 80) AS title, '' AS details, created_at FROM notes WHERE account_id = ?
            UNION ALL
            SELECT 'event' AS type, id, title, substr(details, 1, 80) AS details, created_at FROM events WHERE account_id = ?
        ) ORDER BY created_at DESC LIMIT ?`, accountID, accountID, accountID, limit)
	if err != nil {
		return nil, fmt.Errorf("query account activity: %w", err)
	}
	defer rows.Close()

	var activities []Activity
	for rows.Next() {
		var a Activity
		var created string
		var details sql.NullString
		if err := rows.Scan(&a.Type, &a.ID, &a.Title, &details, &created); err != nil {
			return nil, fmt.Errorf("scan activity: %w", err)
		}
		a.Details = nullStringToString(details)
		if t, err := time.Parse(time.RFC3339, created); err == nil {
			a.CreatedAt = t
		}
		activities = append(activities, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return activities, nil
}

// ImportAccountsCSV ingests accounts from a CSV reader.
func (s *Store) ImportAccountsCSV(ctx context.Context, r io.Reader, defaultCreator string, loc *time.Location) (ImportResult, error) {
	result := ImportResult{}
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return result, fmt.Errorf("read header: %w", err)
	}
	index := map[string]int{}
	for i, h := range header {
		key := strings.ToLower(strings.TrimSpace(h))
		if key != "" {
			index[key] = i
		}
	}
	nameIdx, ok := index["name"]
	if !ok {
		return result, fmt.Errorf("csv missing 'name' column")
	}
	locUsed := loc
	if locUsed == nil {
		locUsed = time.Local
	}
	row := 1
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("row %d: %v", row+1, err))
			continue
		}
		row++
		if nameIdx >= len(record) {
			result.Errors = append(result.Errors, fmt.Sprintf("row %d: missing name field", row))
			result.Skipped++
			continue
		}
		name := strings.TrimSpace(record[nameIdx])
		if name == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("row %d: account name required", row))
			result.Skipped++
			continue
		}
		account := Account{Name: name}
		if idx, ok := index["phone"]; ok && idx < len(record) {
			account.Phone = strings.TrimSpace(record[idx])
		}
		if idx, ok := index["address"]; ok && idx < len(record) {
			account.Address = strings.TrimSpace(record[idx])
		}
		if idx, ok := index["email"]; ok && idx < len(record) {
			account.Email = strings.TrimSpace(record[idx])
		}
		if idx, ok := index["decision_maker"]; ok && idx < len(record) {
			account.DecisionMaker = strings.TrimSpace(record[idx])
		}
		creator := defaultCreator
		if idx, ok := index["creator"]; ok && idx < len(record) {
			val := strings.TrimSpace(record[idx])
			if val != "" {
				creator = val
			}
		}
		if creator == "" {
			creator = "Import"
		}
		account.Creator = creator
		if idx, ok := index["created_at"]; ok && idx < len(record) {
			stamp := strings.TrimSpace(record[idx])
			if stamp != "" {
				if parsed, ok := parseImportTime(stamp, locUsed); ok {
					account.CreatedAt = parsed
				}
			}
		}
		if account.CreatedAt.IsZero() {
			account.CreatedAt = time.Now().In(locUsed)
		}
		if err := s.CreateAccount(ctx, &account); err != nil {
			if errors.Is(err, ErrAccountExists) {
				result.Skipped++
				result.Errors = append(result.Errors, fmt.Sprintf("row %d: duplicate account '%s'", row, account.Name))
				continue
			}
			result.Errors = append(result.Errors, fmt.Sprintf("row %d: %v", row, err))
			result.Skipped++
			continue
		}
		result.Created++
	}
	return result, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAccount(rs rowScanner) (Account, error) {
	var a Account
	var phone, address, email, decision sql.NullString
	var created string
	if err := rs.Scan(&a.ID, &a.Name, &phone, &address, &email, &decision, &a.Creator, &created); err != nil {
		return Account{}, err
	}
	a.Phone = nullStringToString(phone)
	a.Address = nullStringToString(address)
	a.Email = nullStringToString(email)
	a.DecisionMaker = nullStringToString(decision)
	if created != "" {
		if t, err := time.Parse(time.RFC3339, created); err == nil {
			a.CreatedAt = t
		}
	}
	return a, nil
}

func nullStringToString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func parseImportTime(value string, loc *time.Location) (time.Time, bool) {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, value, loc); err == nil {
			return t, true
		}
	}
	if t, err := time.Parse(time.RFC1123, value); err == nil {
		return t.In(loc), true
	}
	return time.Time{}, false
}

func nullString(s string) interface{} {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return s
}

func nullInt64(v sql.NullInt64) interface{} {
	if !v.Valid {
		return nil
	}
	return v.Int64
}

func isUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "unique")
}

// SplitEvents groups events relative to a reference time.
func SplitEvents(events []Event, now time.Time) (today, upcoming, past []Event) {
	loc := now.Location()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	endOfDay := startOfDay.Add(24 * time.Hour)

	for _, e := range events {
		t := e.EventTime.In(loc)
		switch {
		case !t.Before(startOfDay) && t.Before(endOfDay):
			today = append(today, e)
		case t.After(now):
			upcoming = append(upcoming, e)
		default:
			past = append(past, e)
		}
	}

	sort.Slice(today, func(i, j int) bool { return today[i].EventTime.Before(today[j].EventTime) })
	sort.Slice(upcoming, func(i, j int) bool { return upcoming[i].EventTime.Before(upcoming[j].EventTime) })
	sort.Slice(past, func(i, j int) bool { return past[i].EventTime.After(past[j].EventTime) })

	return today, upcoming, past
}
