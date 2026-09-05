import { createApp } from "vue";
import App from "./App.vue";
import { router } from "./router";

import "@fontsource-variable/inter";
import "@fontsource/eb-garamond/400.css";
import "@fontsource/eb-garamond/600.css";
import "./style.css";

createApp(App).use(router).mount("#app");
