package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"mikuchrono/internal/models"
)

func scanCategory(row interface{ Scan(...any) error }) (models.Category, error) {
	var c models.Category
	err := row.Scan(&c.ID, &c.Name, &c.Icon, &c.Color, &c.SortOrder, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	if err != nil {
		return c, err
	}
	return c, nil
}

const categoryCols = `id, name, icon, color, sort_order, created_at, updated_at`

// ListCategories returns every category ordered by sort_order, then id.
func (s *Store) ListCategories() ([]models.Category, error) {
	q := `SELECT ` + categoryCols + ` FROM categories ORDER BY sort_order, id`
	rows, err := s.db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()
	var out []models.Category
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetCategory fetches one category by id.
func (s *Store) GetCategory(id int64) (models.Category, error) {
	c, err := scanCategory(s.db.QueryRow(`SELECT `+categoryCols+` FROM categories WHERE id = ?`, id))
	if err != nil {
		return c, fmt.Errorf("get category %d: %w", id, err)
	}
	return c, nil
}

// CreateCategory inserts a new category.
func (s *Store) CreateCategory(c models.Category) (models.Category, error) {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return c, validationf("类别名称不能为空")
	}
	if c.Color == "" {
		c.Color = "#cc785c"
	}
	now := NowString()
	res, err := s.db.Exec(
		`INSERT INTO categories(name,icon,color,sort_order,created_at,updated_at) VALUES(?,?,?,?,?,?)`,
		c.Name, c.Icon, c.Color, c.SortOrder, now, now,
	)
	if err != nil {
		return c, fmt.Errorf("create category: %w", err)
	}
	c.ID, _ = res.LastInsertId()
	c.CreatedAt, c.UpdatedAt = now, now
	return c, nil
}

// UpdateCategory updates name/icon/color/sort for a category.
func (s *Store) UpdateCategory(c models.Category) error {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return validationf("类别名称不能为空")
	}
	if c.Color == "" {
		c.Color = "#cc785c"
	}
	res, err := s.db.Exec(
		`UPDATE categories SET name=?, icon=?, color=?, sort_order=?, updated_at=? WHERE id=?`,
		c.Name, c.Icon, c.Color, c.SortOrder, NowString(), c.ID,
	)
	if err != nil {
		return fmt.Errorf("update category %d: %w", c.ID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteCategory removes a category; its activities become uncategorized
// via the ON DELETE SET NULL foreign key.
func (s *Store) DeleteCategory(id int64) error {
	res, err := s.db.Exec(`DELETE FROM categories WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete category %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
