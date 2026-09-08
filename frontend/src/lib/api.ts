// Central re-export of the generated Wails bindings so views/components
// import from one place.
import * as TimerService from "../../bindings/mikuchrono/internal/services/timerservice";
import * as ActivityService from "../../bindings/mikuchrono/internal/services/activityservice";
import * as CategoryService from "../../bindings/mikuchrono/internal/services/categoryservice";
import * as EntryService from "../../bindings/mikuchrono/internal/services/entryservice";
import * as StatsService from "../../bindings/mikuchrono/internal/services/statsservice";
import * as DataService from "../../bindings/mikuchrono/internal/services/dataservice";
import * as PetService from "../../bindings/mikuchrono/internal/services/petservice";
import type {
  Activity,
  ActivityStat,
  ActivityTotal,
  Category,
  DayBucket,
  Entry,
  EntryFilter,
  EntryList,
  MonthBucket,
  PetPosition,
  StreakInfo,
  TimerState,
} from "../../bindings/mikuchrono/internal/models/models";

export {
  TimerService,
  ActivityService,
  CategoryService,
  EntryService,
  StatsService,
  DataService,
  PetService,
};
export type {
  Activity,
  ActivityStat,
  ActivityTotal,
  Category,
  DayBucket,
  Entry,
  EntryFilter,
  EntryList,
  MonthBucket,
  PetPosition,
  StreakInfo,
  TimerState,
};
