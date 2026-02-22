CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS jobs (
  id TEXT PRIMARY KEY,
  status TEXT NOT NULL,
  prompt_raw TEXT NOT NULL,
  prompt_final TEXT NOT NULL,
  negative_prompt TEXT NOT NULL,
  comfy_prompt_id TEXT NOT NULL DEFAULT '',
  output_file TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
