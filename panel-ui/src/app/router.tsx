import { createBrowserRouter, Navigate } from "react-router";
import { AuthenticatorPage, CodePage, InvitePage, ForgotPage, ResetPage, LoginPage, RequireAuth } from "@/features/auth";
import { AccountPage } from "@/features/account";
import { TenantsPage } from "@/features/tenants";
import { SiteLayout, SitesPage } from "@/features/sites";
import { ConnectorsPage } from "@/features/connectors";
import { IsolationPage } from "@/features/exposure";
import { CertificatePage } from "@/features/certificate";
import { DiscoveryPage } from "@/features/discovery";
import { SettingsPage } from "@/features/settings";
import { MapsPage, NewVersionPage, VersionPage } from "@/features/maps";
import { UsersPage } from "@/features/users";
import { AuditPage } from "@/features/audit";
import { LogsPage } from "@/features/logs";
import { PlatformStatusPage, StatusPage } from "@/features/status";
import { LoginTab } from "@/features/sitelogin";
import { SecurityTab } from "@/features/security";
import { HelpPage } from "@/features/help";
import { Shell } from "./Shell";
import { Home } from "./Home";

export const router = createBrowserRouter([
  { path: "/login", element: <LoginPage /> },
  { path: "/login/code", element: <CodePage /> },
  { path: "/login/authenticator", element: <AuthenticatorPage /> },
  { path: "/invite/:token", element: <InvitePage /> },
  { path: "/forgot", element: <ForgotPage /> },
  { path: "/reset/:token", element: <ResetPage /> },
  {
    element: <RequireAuth><Shell /></RequireAuth>,
    children: [
      { index: true, element: <Home /> },
      { path: "tenants", element: <TenantsPage /> },
      { path: "activity", element: <AuditPage /> },
      { path: "account", element: <AccountPage /> },
      { path: "help/:topic?", element: <HelpPage /> },
      { path: "t/:tenant", element: <SitesPage /> },
      { path: "t/:tenant/people", element: <UsersPage /> },
      { path: "t/:tenant/activity", element: <AuditPage /> },
      { path: "t/:tenant/logs", element: <LogsPage /> },
      { path: "t/:tenant/status", element: <StatusPage /> },
      { path: "status", element: <PlatformStatusPage /> },
      { path: "t/:tenant/connectors", element: <ConnectorsPage /> },
      { path: "t/:tenant/settings", element: <SettingsPage /> },
      {
        path: "t/:tenant/sites/:app",
        element: <SiteLayout />,
        children: [
          { index: true, element: <Navigate to="discovery" replace /> },
          { path: "maps", element: <MapsPage /> },
          { path: "maps/new", element: <NewVersionPage /> },
          { path: "maps/:version", element: <VersionPage /> },
          { path: "discovery", element: <DiscoveryPage /> },
          { path: "isolation", element: <IsolationPage /> },
          { path: "certificate", element: <CertificatePage /> },
          { path: "login", element: <LoginTab /> },
          { path: "security", element: <SecurityTab /> },
        ],
      },
      { path: "*", element: <p className="text-ink-soft">This page does not exist.</p> },
    ],
  },
]);
