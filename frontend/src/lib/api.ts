// Central re-export of the generated Wails bindings so views/components
// import from one place.
import * as TimerService from "../../bindings/mikuchrono/internal/services/timerservice";
import * as ActivityService from "../../bindings/mikuchrono/internal/services/activityservice";
import * as CategoryService from "../../bindings/mikuchrono/internal/services/categoryservice";
import * as EntryService from "../../bindings/mikuchrono/internal/services/entryservice";
import * as StatsService from "../../bindings/mikuchrono/internal/services/statsservice";
import * as DataService from "../../bindings/mikuchrono/internal/services/dataservice";
import * as BallService from "../../bindings/mikuchrono/internal/services/ballservice";
import type {
  Activity,
  ActivityStat,
  ActivityTotal,
  BallPosition,
  Category,
  DayBucket,
  Entry,
  EntryFilter,
  EntryList,
  StreakInfo,
  TimerState,
} from "../../bindings/mikuchrono/internal/models/models";

export { TimerService, ActivityService, CategoryService, EntryService, StatsService, DataService, BallService };
export type { Activity, ActivityStat, ActivityTotal, BallPosition, Category, DayBucket, Entry, EntryFilter, EntryList, StreakInfo, TimerState };
