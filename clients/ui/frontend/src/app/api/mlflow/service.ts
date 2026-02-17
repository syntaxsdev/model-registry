/**
 * MLflow REST API service for registering models from the KF Catalog.
 *
*
* 
 *
 * This service is used as an alternative registration target when the user
 * wants to register a catalog model into MLflow instead of the KF Model Registry.
 */

// Proxied through webpack dev server -> localhost:5001
// See config/webpack.dev.js has the proxy config
const MLFLOW_BASE_URL = '/mlflow-api';

interface MlflowTag {
  key: string;
  value: string;
}

export interface MlflowRegisteredModel {
  name: string;
  creation_timestamp: number;
  last_updated_timestamp: number;
  description?: string;
  tags?: MlflowTag[];
}

export interface MlflowModelVersion {
  name: string;
  version: string;
  creation_timestamp: number;
  last_updated_timestamp: number;
  description?: string;
  source?: string;
  tags?: MlflowTag[];
  status: string;
}

export interface MlflowExperiment {
  experiment_id: string;
  name: string;
}

export interface MlflowRun {
  info: {
    run_id: string;
    experiment_id: string;
    status: string;
  };
}

async function mlflowFetch<T>(
  endpoint: string,
  method: string,
  body?: Record<string, unknown>,
): Promise<T> {
  const response = await fetch(`${MLFLOW_BASE_URL}${endpoint}`, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(`MLflow API error (${response.status}): ${errorText}`);
  }
  return response.json() as Promise<T>;
}

// --- Registered Models ---

export async function createMlflowRegisteredModel(
  name: string,
  description?: string,
  tags?: MlflowTag[],
): Promise<MlflowRegisteredModel> {
  const body: Record<string, unknown> = { name };
  if (description) {
    body.description = description;
  }
  if (tags && tags.length > 0) {
    body.tags = tags;
  }
  const result = await mlflowFetch<{ registered_model: MlflowRegisteredModel }>(
    '/api/2.0/mlflow/registered-models/create',
    'POST',
    body,
  );
  return result.registered_model;
}

export async function setMlflowRegisteredModelTag(
  name: string,
  key: string,
  value: string,
): Promise<void> {
  await mlflowFetch('/api/2.0/mlflow/registered-models/set-tag', 'POST', { name, key, value });
}

// --- Model Versions ---

export async function createMlflowModelVersion(
  name: string,
  source: string,
  description?: string,
  tags?: MlflowTag[],
  runId?: string,
): Promise<MlflowModelVersion> {
  const body: Record<string, unknown> = { name, source };
  if (description) {
    body.description = description;
  }
  if (tags && tags.length > 0) {
    body.tags = tags;
  }
  if (runId) {
    body.run_id = runId;
  }
  const result = await mlflowFetch<{ model_version: MlflowModelVersion }>(
    '/api/2.0/mlflow/model-versions/create',
    'POST',
    body,
  );
  return result.model_version;
}

export async function setMlflowModelVersionTag(
  name: string,
  version: string,
  key: string,
  value: string,
): Promise<void> {
  await mlflowFetch('/api/2.0/mlflow/model-versions/set-tag', 'POST', {
    name,
    version,
    key,
    value,
  });
}

// --- Experiments & Runs (for lineage tracking) ---

export async function createMlflowExperiment(name: string): Promise<string> {
  const result = await mlflowFetch<{ experiment_id: string }>(
    '/api/2.0/mlflow/experiments/create',
    'POST',
    { name },
  );
  return result.experiment_id;
}

export async function getOrCreateMlflowExperiment(name: string): Promise<string> {
  try {
    return await createMlflowExperiment(name);
  } catch {
    // Experiment may already exist, try to get it
    const result = await mlflowFetch<{ experiment: MlflowExperiment }>(
      `/api/2.0/mlflow/experiments/get-by-name?experiment_name=${encodeURIComponent(name)}`,
      'GET',
    );
    return result.experiment.experiment_id;
  }
}

export async function createMlflowRun(
  experimentId: string,
  runName: string,
  tags?: MlflowTag[],
): Promise<MlflowRun> {
  const result = await mlflowFetch<{ run: MlflowRun }>('/api/2.0/mlflow/runs/create', 'POST', {
    experiment_id: experimentId,
    run_name: runName,
    tags: tags || [],
  });
  return result.run;
}

export async function logMlflowParam(
  runId: string,
  key: string,
  value: string,
): Promise<void> {
  await mlflowFetch('/api/2.0/mlflow/runs/log-parameter', 'POST', {
    run_id: runId,
    key,
    value,
  });
}

export async function updateMlflowRun(
  runId: string,
  status: string,
): Promise<void> {
  await mlflowFetch('/api/2.0/mlflow/runs/update', 'POST', {
    run_id: runId,
    status,
    end_time: Date.now(),
  });
}
