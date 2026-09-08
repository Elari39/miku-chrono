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
      redirect: "/",
    },
    {
      path: "/settings",
      name: "settings",
      component: () => import("./views/SettingsView.vue"),
      meta: { title: "设置" },
    },
    // Full-window page for the desktop pet (transparent frameless window).
    {
      path: "/pet",
      name: "pet",
      component: () => import("./views/PetView.vue"),
      meta: { title: "桌宠" },
    },
  ],
});
