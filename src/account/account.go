package account

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"justcpc-online-judge/src/store"
)

type Account struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type AdminAccount struct {
	Account
	Password string `json:"password"`
}

type Service struct{ Store *store.Store }

func (s Service) EnsureAdmin(ctx context.Context, username, password string) error {
	var count int
	if err := s.Store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts WHERE role = 'admin'`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	if username == "" || password == "" {
		return errors.New("first startup requires JUSTCPC_INITIAL_ADMIN_PASSWORD")
	}
	_, err := s.Create(ctx, username, password, "admin")
	return err
}

func (s Service) Create(ctx context.Context, username, password, role string) (Account, error) {
	if password == "" {
		password = "123456"
	}
	if username == "" || (role != "admin" && role != "user") {
		return Account{}, errors.New("invalid account")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return Account{}, err
	}
	now := store.Now()
	result, err := s.Store.DB.ExecContext(ctx, `INSERT INTO accounts(username,password_hash,password_text,role,created_at,updated_at) VALUES(?,?,?,?,?,?)`, username, string(hash), password, role, now, now)
	if err != nil {
		return Account{}, fmt.Errorf("create account: %w", err)
	}
	id, _ := result.LastInsertId()
	return Account{ID: id, Username: username, Role: role}, nil
}

func (s Service) Authenticate(ctx context.Context, username, password string) (Account, error) {
	var account Account
	var hash string
	err := s.Store.DB.QueryRowContext(ctx, `SELECT id,username,password_hash,role FROM accounts WHERE username=?`, username).Scan(&account.ID, &account.Username, &hash, &account.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, errors.New("username or password is incorrect")
	}
	if err != nil {
		return Account{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return Account{}, errors.New("username or password is incorrect")
	}
	return account, nil
}

func (s Service) Get(ctx context.Context, id int64) (Account, error) {
	var account Account
	err := s.Store.DB.QueryRowContext(ctx, `SELECT id,username,role FROM accounts WHERE id=?`, id).Scan(&account.ID, &account.Username, &account.Role)
	return account, err
}

func (s Service) ListForAdmin(ctx context.Context) ([]AdminAccount, error) {
	rows, err := s.Store.DB.QueryContext(ctx, `SELECT id,username,role,password_text FROM accounts ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var accounts []AdminAccount
	for rows.Next() {
		var account AdminAccount
		if err := rows.Scan(&account.ID, &account.Username, &account.Role, &account.Password); err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

func (s Service) Update(ctx context.Context, id int64, username, password, role string) (Account, error) {
	if id <= 0 || username == "" || password == "" || (role != "admin" && role != "user") {
		return Account{}, errors.New("invalid account")
	}
	var oldRole string
	if err := s.Store.DB.QueryRowContext(ctx, `SELECT role FROM accounts WHERE id=?`, id).Scan(&oldRole); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Account{}, errors.New("account not found")
		}
		return Account{}, err
	}
	if oldRole == "admin" && role != "admin" {
		var adminCount int
		if err := s.Store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts WHERE role='admin'`).Scan(&adminCount); err != nil {
			return Account{}, err
		}
		if adminCount <= 1 {
			return Account{}, errors.New("at least one administrator must remain")
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return Account{}, err
	}
	_, err = s.Store.DB.ExecContext(ctx, `UPDATE accounts SET username=?,password_hash=?,password_text=?,role=?,updated_at=? WHERE id=?`, username, string(hash), password, role, store.Now(), id)
	if err != nil {
		return Account{}, fmt.Errorf("update account: %w", err)
	}
	return Account{ID: id, Username: username, Role: role}, nil
}
