import { createBrowserRouter, Navigate } from "react-router";
import { LoginPage, RequireAuth } from "@/features/auth";
import { TenantsPage } from "@/features/tenants";
import { SiteLayout, SitesPage } from "@/features/sites";
import { ConnectorsPage } from "@/features/connectors";
import { MapsPage, NewVersionPage, VersionPage } from "@/features/maps";
import { UsersPage } from "@/features/users";
import { AuditPage } from "@/features/audit";
import { Shell } from "./Shell";
import { Home } from "./Home";

export const router = createBrowserRouter([
  { path: "/login", element: <LoginPage /> },
  {
    element: <RequireAuth><Shell /></RequireAuth>,
    children: [
      { index: true, element: <Home /> },
      { path: "tenants", element: <TenantsPage /> },
      { path: "activity", element: <AuditPage /> },
      { path: "t/:tenant", element: <SitesPage /> },
      { path: "t/:tenant/people", element: <UsersPage /> },
      { path: "t/:tenant/activity", element: <AuditPage /> },
      {
        path: "t/:tenant/sites/:app",
        element: <SiteLayout />,
        children: [
          { index: true, element: <Navigate to="maps" replace /> },
          { path: "maps", element: <MapsPage /> },
          { path: "maps/new", element: <NewVersionPage /> },
          { path: "maps/:version", element: <VersionPage /> },
          { path: "connectors", element: <ConnectorsPage /> },
        ],
      },
      { path: "*", element: <p className="text-ink-soft">This page does not exist.</p> },
    ],
  },
]);
