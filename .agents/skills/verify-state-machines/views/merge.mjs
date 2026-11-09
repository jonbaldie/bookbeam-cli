export const title = "Partial updates can overwrite newer values";
export const state = v => {
 if(v.phase === "edits") return "Requested edits";
 if(v.phase === "error") return "Read failed";
 if(v.phase === "snapshot") return v.changed ? "Snapshot is stale" : "Existing values fetched";
 if(v.phase === "ready") return v.changed ? "Stale payload ready" : "Payload ready";
 if(v.phase === "done") return v.missing && v.changed ? "Newer value overwritten" : "Update sent";
 return v.phase;
};
export const step = action => ({ReadExisting:"fetch missing values",FullEdit:"all values supplied",Merge:"fill missing values",ConcurrentEdit:"someone edits the preserved field",Put:"send full update"})[action] ?? null;
