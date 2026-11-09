export const title = "Deletion authorization";
export const state = v => ({validate:"Validate IDs",authorize:"Decide whether to proceed",delete:"Deletion authorized",done:v.deleted ? "Delete sent" : "No delete sent"})[v.phase];
export const step = (action,from,to) => ({Validate:from.valid ? "IDs accepted" : "invalid IDs",Authorize:to.authorized ? (from.force ? "force flag" : from.json ? "JSON script mode" : "answer yes") : "refuse or end input",Delete:"send DELETE"})[action] ?? null;
