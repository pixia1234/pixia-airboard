/// <reference types="vite/client" />

interface AirboardBootstrap {
  page: "user" | "admin";
  title: string;
  description: string;
  adminPath: string;
  apiBase: string;
  appUrl: string;
}

interface Window {
  __AIRBOARD_BOOTSTRAP__?: AirboardBootstrap;
}
