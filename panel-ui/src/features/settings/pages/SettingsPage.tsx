import { useParams } from "react-router";
import { Notice, PageHeader } from "@/shared/ui";
import { describeError } from "@/shared/api/errors";
import { useLLM } from "../hooks/useLLM";
import { LLMForm } from "../components/LLMForm";
import { useAlerts } from "../hooks/useAlerts";
import { AlertsForm } from "../components/AlertsForm";
import { useAssistantSettings } from "../hooks/useAssistantSettings";
import { AssistantForm } from "../components/AssistantForm";

/** Tenant settings: email alerts, the AI model, how long the assistant keeps conversations. */
export function SettingsPage() {
  const tenant = useParams().tenant ?? "";
  const llm = useLLM(tenant);
  const alerts = useAlerts(tenant);
  const assistant = useAssistantSettings(tenant);
  if (!llm.data && !llm.isError) return null;
  return (
    <div className="flex flex-col gap-6">
      <PageHeader title="Settings" />
      {alerts.data ? <AlertsForm key={String(alerts.dataUpdatedAt)} tenant={tenant} current={alerts.data} /> : null}
      {llm.isError ? <Notice tone="error">{describeError(llm.error)}</Notice> : null}
      {llm.data ? <LLMForm key={llm.data.provider ?? "none"} tenant={tenant} current={llm.data} /> : null}
      {assistant.data ? <AssistantForm key={assistant.data.keep_days} tenant={tenant} keepDays={assistant.data.keep_days} /> : null}
    </div>
  );
}
