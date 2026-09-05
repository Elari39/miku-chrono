import { createRouter, createWebHashHistory } from "vue-router";

// Hash history is required: production assets are served over file:// where
// real URLs cannot be rewritten.
export const router = createRouter({
  history: createWebHashHistory(),
  scrollBehavior: () => ({ top: 0 }),
  routes: [
    {
      path: "/",
      name: "checkin",
      component: () => import("./views/CheckInView.vue"),
      meta: { title: "打卡" },
    },
    {
      path: "/stats",
      name: "stats",
      component: () => import("./views/StatsView.vue"),
      meta: { title: "统计" },
    },
    {
      path: "/records",
      name: "records",
      component: () => import("./views/RecordsView.vue"),
      meta: { title: "记录" },
    },
    {
      path: "/activities",
      name: "activities",
      component: () => import("./views/ActivitiesView.vue"),
      meta: { title: "活动" },
    },
    {
      path: "/settings",
      name: "settings",
      component: () => import("./views/SettingsView.vue"),
      meta: { title: "设置" },
    },
    // Full-window page for the floating ball (transparent frameless window).
    { path: "/ball", name: "ball", component: () => import("./views/BallView.vue") },
  ],
});
