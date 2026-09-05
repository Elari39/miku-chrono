package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"mikuchrono/internal/models"
)

// idPtr returns a pointer to v, for nullable category_id fields.
func idPtr(v int64) *int64 { return &v }

func TestCategoryCRUD(t *testing.T) {
	s := newTestStore(t)

	// Create two categories; display order follows sort_order.
	c1, err := s.CreateCategory(models.Category{Name: "学习", Icon: "📚", Color: "#cc785c", SortOrder: 1})
	if err != nil {
		t.Fatalf("create c1: %v", err)
	}
	c2, err := s.CreateCategory(models.Category{Name: "健康", Icon: "💪", Color: "#5db8a6", SortOrder: 2})
	if err != nil {
		t.Fatalf("create c2: %v", err)
	}
	if c1.ID == c2.ID || c1.CreatedAt == "" || c2.CreatedAt == "" {
		t.Fatalf("created categories malformed: %+v %+v", c1, c2)
	}

	list, err := s.ListCategories()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Name != "学习" || list[1].Name != "健康" {
		t.Fatalf("list: %+v", list)
	}

	// Update then verify persisted.
	c1.Name = "学习提升"
	c1.Icon = "🎓"
	c1.SortOrder = 9
	if err := s.UpdateCategory(c1); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := s.GetCategory(c1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "学习提升" || got.Icon != "🎓" || got.SortOrder != 9 {
		t.Fatalf("after update: %+v", got)
	}

	// Defaults: empty color falls back to the palette default.
	c3, err := s.CreateCategory(models.Category{Name: "娱乐"})
	if err != nil {
		t.Fatal(err)
	}
	if c3.Color != "#cc785c" {
		t.Fatalf("default color: %q", c3.Color)
	}

	if err := s.DeleteCategory(c2.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.GetCategory(c2.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted category should be gone, got %v", err)
	}
	if err := s.DeleteCategory(999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleting unknown category must report not found, got %v", err)
	}
}

func TestCategoryValidation(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.CreateCategory(models.Category{Name: "   "}); !errors.Is(err, ErrValidation) {
		t.Fatalf("blank name must be rejected, got %v", err)
	}
	c, err := s.CreateCategory(models.Category{Name: "学习"})
	if err != nil {
		t.Fatal(err)
	}
	c.Name = "  "
	if err := s.UpdateCategory(c); !errors.Is(err, ErrValidation) {
		t.Fatalf("blank update name must be rejected, got %v", err)
	}
}

func TestDeleteCategoryUncategorizesActivities(t *testing.T) {
	s := newTestStore(t)

	cat, err := s.CreateCategory(models.Category{Name: "学习"})
	if err != nil {
		t.Fatal(err)
	}
	act, err := s.CreateActivity(models.Activity{Name: "读书", Color: "#cc785c", CategoryID: idPtr(cat.ID)})
	if err != nil {
		t.Fatal(err)
	}

	// Referenced activity rounds-trips its category id.
	got, err := s.GetActivity(act.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CategoryID == nil || *got.CategoryID != cat.ID {
		t.Fatalf("activity category: %+v", got.CategoryID)
	}

	// Deleting the category must NOT delete the activity or its data; it
	// only clears the link (ON DELETE SET NULL).
	now := time.Date(2025, 9, 1, 12, 0, 0, 0, time.Local)
	if _, err := s.CreateManualEntry(act.ID, "2025-09-01T10:00:00+08:00", "2025-09-01T11:00:00+08:00", "", now); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteCategory(cat.ID); err != nil {
		t.Fatal(err)
	}
	act, err = s.GetActivity(act.ID)
	if err != nil {
		t.Fatalf("activity must survive category deletion: %v", err)
	}
	if act.CategoryID != nil {
		t.Fatalf("activity category_id must be NULL, got %+v", act.CategoryID)
	}
	if list, _ := s.ListEntries(models.EntryFilter{ActivityID: &act.ID}); list.Total != 1 {
		t.Fatalf("entries must survive, got %d", list.Total)
	}
}

func TestActivityCreateUpdateWithCategory(t *testing.T) {
	s := newTestStore(t)

	study, err := s.CreateCategory(models.Category{Name: "学习"})
	if err != nil {
		t.Fatal(err)
	}
	health, err := s.CreateCategory(models.Category{Name: "健康"})
	if err != nil {
		t.Fatal(err)
	}

	act, err := s.CreateActivity(models.Activity{Name: "晨跑", Color: "#5db8a6", CategoryID: idPtr(study.ID)})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetActivity(act.ID)
	if got.CategoryID == nil || *got.CategoryID != study.ID {
		t.Fatalf("create with category: %+v", got.CategoryID)
	}

	// Move to another category.
	act.CategoryID = idPtr(health.ID)
	if err := s.UpdateActivity(act); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetActivity(act.ID)
	if got.CategoryID == nil || *got.CategoryID != health.ID {
		t.Fatalf("update category: %+v", got.CategoryID)
	}

	// Clear the category back to uncategorized (nil pointer).
	act.CategoryID = nil
	if err := s.UpdateActivity(act); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetActivity(act.ID)
	if got.CategoryID != nil {
		t.Fatalf("clearing category must store NULL, got %+v", got.CategoryID)
	}

	// Deleting the category an activity points at must not break activity
	// rows (SET NULL safety even for stale references).
	stale, err := s.CreateActivity(models.Activity{Name: "临时", CategoryID: idPtr(999)})
	if err == nil {
		// FK enforced: inserting a dangling category id fails.
		_ = stale
		t.Fatal("activity with unknown category must be rejected")
	}
}

// TestMigrateV1toV2PreservesData builds a database at schema v1 (as shipped
// before categories existed), inserts data, then reopens it with Open() to
// exercise the real migration path.
func TestMigrateV1toV2PreservesData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
		INSERT INTO meta(key,value) VALUES('schema_version','1');
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(migrations[0]); err != nil {
		t.Fatalf("apply v1 schema: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO activities(name,color,icon,daily_goal_minutes,sort_order,archived,created_at,updated_at)
		 VALUES('旧活动','#cc785c','',60,1,0,'2025-01-01T00:00:00+08:00','2025-01-01T00:00:00+08:00')`,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO entries(activity_id,started_at,ended_at,duration_seconds,note,source,created_at,updated_at)
		 VALUES(1,'2025-01-01T09:00:00+08:00','2025-01-01T10:00:00+08:00',3600,'','timer','2025-01-01T00:00:00+08:00','2025-01-01T00:00:00+08:00')`,
	); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	// Reopen through the public path: v2 migration must run.
	s, err := Open(path)
	if err != nil {
		t.Fatalf("reopen after migration: %v", err)
	}
	defer func() { _ = s.Close() }()

	// Old data survives.
	act, err := s.GetActivity(1)
	if err != nil {
		t.Fatalf("legacy activity: %v", err)
	}
	if act.Name != "旧活动" || act.DailyGoalMinutes != 60 {
		t.Fatalf("legacy activity mangled: %+v", act)
	}
	if act.CategoryID != nil {
		t.Fatalf("legacy activity must start uncategorized: %+v", act.CategoryID)
	}

	// New schema is usable: categories can be created and assigned.
	cat, err := s.CreateCategory(models.Category{Name: "旧活动类别"})
	if err != nil {
		t.Fatal(err)
	}
	act.CategoryID = idPtr(cat.ID)
	if err := s.UpdateActivity(act); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetActivity(1)
	if got.CategoryID == nil || *got.CategoryID != cat.ID {
		t.Fatalf("assign category after migration: %+v", got.CategoryID)
	}
}
