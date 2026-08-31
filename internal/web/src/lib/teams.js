// Creating a team is an instance administrator's job, and the only entry to a
// first team on a new instance — so the sidebar switcher offers it too.

import { api } from "./api.js";
import { reloadContext } from "./state.svelte.js";
import { navigate } from "./router.svelte.js";
import { openModal } from "./ui.svelte.js";
import FormDialog from "../dialogs/FormDialog.svelte";

export async function newTeam() {
  const res = await openModal(FormDialog, {
    title: "New team",
    action: "Create",
    fields: [
      { name: "name", label: "Name" },
      { name: "shortCode", label: "Short code", placeholder: "SOL", maxlength: 8 },
    ],
    explain: "The short code prefixes every identifier this team mints. It is set " +
      "once, never changed, and never reused — not even after archival.",
  });
  if (!res) return null;
  const team = await api.createTeam(res.name, res.shortCode);
  await reloadContext(team.id);
  navigate(`/t/${team.id}/board`);
  return team;
}
