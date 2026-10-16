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

const DefaultAdminPassword = "12345678"

// EnsureAdmin preserves existing passwords and repairs a previously demoted
// built-in administrator. The bool reports creation, for first-login guidance.
func (s Service) EnsureAdmin(ctx context.Context) (bool, error) {
	var id int64
	var role string
	err := s.Store.DB.QueryRowContext(ctx, `SELECT id,role FROM accounts WHERE username='admin'`).Scan(&id, &role)
	if err == nil {
		if role != "admin" {
			_, err = s.Store.DB.ExecContext(ctx, `UPDATE accounts SET role='admin',updated_at=? WHERE id=?`, store.Now(), id)
		}
		return false, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	_, err = s.Create(ctx, "admin", DefaultAdminPassword, "admin")
	return err == nil, err
}

func (s Service) Create(ctx context.Context, username, password, role string) (Account, error) {
	if password == "" {
		password = "123456"
	}
	if username == "" || (role != "admin" && role != "user") {
		return Account{}, errors.New("invalid account")
	}
	if username == "admin" && role != "admin" {
		return Account{}, errors.New("admin 账号只能是管理员")
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
	var oldUsername, oldRole string
	if err := s.Store.DB.QueryRowContext(ctx, `SELECT username,role FROM accounts WHERE id=?`, id).Scan(&oldUsername, &oldRole); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Account{}, errors.New("account not found")
		}
		return Account{}, err
	}
	if oldUsername == "admin" && username != "admin" {
		return Account{}, errors.New("内置 admin 账号不能改名")
	}
	if username == "admin" && role != "admin" {
		return Account{}, errors.New("admin 账号只能是管理员")
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
