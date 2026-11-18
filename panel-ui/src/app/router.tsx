import { createBrowserRouter, Navigate } from "react-router";
import { AuthenticatorPage, CodePage, InvitePage, LoginPage, RequireAuth } from "@/features/auth";
import { AccountPage } from "@/features/account";
import { TenantsPage } from "@/features/tenants";
import { SiteLayout, SitesPage } from "@/features/sites";
import { ConnectorsPage } from "@/features/connectors";
import { IsolationPage } from "@/features/exposure";
import { DiscoveryPage } from "@/features/discovery";
import { SettingsPage } from "@/features/settings";
import { MapsPage, NewVersionPage, VersionPage } from "@/features/maps";
import { UsersPage } from "@/features/users";
import { AuditPage } from "@/features/audit";
import { Shell } from "./Shell";
import { Home } from "./Home";

export const router = createBrowserRouter([
  { path: "/login", element: <LoginPage /> },
  { path: "/login/code", element: <CodePage /> },
  { path: "/login/authenticator", element: <AuthenticatorPage /> },
  { path: "/invite/:token", element: <InvitePage /> },
  {
    element: <RequireAuth><Shell /></RequireAuth>,
    children: [
      { index: true, element: <Home /> },
      { path: "tenants", element: <TenantsPage /> },
      { path: "activity", element: <AuditPage /> },
      { path: "account", element: <AccountPage /> },
      { path: "t/:tenant", element: <SitesPage /> },
      { path: "t/:tenant/people", element: <UsersPage /> },
      { path: "t/:tenant/activity", element: <AuditPage /> },
      { path: "t/:tenant/connectors", element: <ConnectorsPage /> },
      { path: "t/:tenant/settings", element: <SettingsPage /> },
      {
        path: "t/:tenant/sites/:app",
        element: <SiteLayout />,
        children: [
          { index: true, element: <Navigate to="maps" replace /> },
          { path: "maps", element: <MapsPage /> },
          { path: "maps/new", element: <NewVersionPage /> },
          { path: "maps/:version", element: <VersionPage /> },
          { path: "discovery", element: <DiscoveryPage /> },
          { path: "isolation", element: <IsolationPage /> },
        ],
      },
      { path: "*", element: <p className="text-ink-soft">This page does not exist.</p> },
    ],
  },
]);
