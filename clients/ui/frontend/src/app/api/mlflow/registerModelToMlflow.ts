/**
 * Register a model from the KF Catalog into MLflow.
 *
 * This function takes the same form data used for KF Model Registry registration
 * and creates the equivalent resources in MLflow:
 *
 * KF Model Registry              →  MLflow
 * ─────────────────────────────────────────────
 * RegisteredModel                 →  Registered Model
 * ModelVersion                    →  Model Version (with source URI)
 * ModelArtifact (uri, format)     →  (included in Model Version source)
 * customProperties                →  Tags on model + version
 * (nothing)                       →  Experiment + Run (for lineage tracking)
 */

import {
  RegisterCatalogModelFormData,
  ModelLocationType,
} from '~/app/pages/modelRegistry/screens/RegisterModel/useRegisterModelData';
import { objectStorageFieldsToUri } from '~/app/utils';
import {
  createMlflowRegisteredModel,
  setMlflowRegisteredModelTag,
  createMlflowModelVersion,
  setMlflowModelVersionTag,
  getOrCreateMlflowExperiment,
  createMlflowRun,
  logMlflowParam,
  updateMlflowRun,
  MlflowRegisteredModel,
  MlflowModelVersion,
} from './service';

export interface MlflowRegistrationResult {
  registeredModel?: MlflowRegisteredModel;
  modelVersion?: MlflowModelVersion;
  runId?: string;
  errors: { [key: string]: Error | undefined };
}

/**
 * Converts KF Model Registry custom properties to MLflow tags.
 * Custom properties are stored as { key: { string_value: "val", metadataType: "..." } }
 * MLflow tags are { key: string, value: string }.
 */
function customPropertiesToTags(
  customProperties: Record<string, { string_value?: string; int_value?: string; metadataType: string }> | undefined,
): Array<{ key: string; value: string }> {
  if (!customProperties) {
    return [];
  }
  return Object.entries(customProperties)
    .filter(([, v]) => v.string_value !== undefined || v.int_value !== undefined)
    .map(([key, v]) => ({
      key,
      value: v.string_value || v.int_value || '',
    }));
}

function getModelUri(formData: RegisterCatalogModelFormData): string {
  if (formData.modelLocationType === ModelLocationType.ObjectStorage) {
    return (
      objectStorageFieldsToUri({
        endpoint: formData.modelLocationEndpoint,
        bucket: formData.modelLocationBucket,
        region: formData.modelLocationRegion,
        path: formData.modelLocationPath,
      }) || ''
    );
  }
  return formData.modelLocationURI;
}

export async function registerModelToMlflow(
  formData: RegisterCatalogModelFormData,
  author: string,
): Promise<MlflowRegistrationResult> {
  const errors: { [key: string]: Error | undefined } = {};
  let registeredModel: MlflowRegisteredModel | undefined;
  let modelVersion: MlflowModelVersion | undefined;
  let runId: string | undefined;

  const modelUri = getModelUri(formData);

  // Step 1: Create an experiment and run for catalog import lineage
  try {
    const experimentId = await getOrCreateMlflowExperiment('catalog-imports');
    const run = await createMlflowRun(experimentId, `catalog-import-${formData.modelName}`, [
      { key: 'mlflow.source.type', value: 'CATALOG' },
      { key: 'mlflow.source.name', value: 'kf-model-catalog' },
      { key: 'catalog.author', value: author },
    ]);
    runId = run.info.run_id;

    // Log catalog metadata as run params
    await logMlflowParam(runId, 'catalog_source', formData.modelRegistry || 'kf-catalog');
    await logMlflowParam(runId, 'model_uri', modelUri);
    if (formData.sourceModelFormat) {
      await logMlflowParam(runId, 'model_format', formData.sourceModelFormat);
    }
    if (formData.sourceModelFormatVersion) {
      await logMlflowParam(runId, 'model_format_version', formData.sourceModelFormatVersion);
    }

    // Log custom properties as params
    const modelTags = customPropertiesToTags(formData.modelCustomProperties);
    for (const tag of modelTags) {
      await logMlflowParam(runId, `catalog.${tag.key}`, tag.value);
    }

    // Mark run as finished
    await updateMlflowRun(runId, 'FINISHED');
  } catch (e) {
    // Non-fatal: experiment/run creation is for lineage, not required
    console.warn('Failed to create MLflow experiment/run for lineage tracking:', e);
  }

  // Step 2: Create the registered model in MLflow
  try {
    const modelTags = customPropertiesToTags(formData.modelCustomProperties);
    registeredModel = await createMlflowRegisteredModel(
      formData.modelName,
      formData.modelDescription,
      modelTags,
    );

    // Set additional tags that don't fit in the create call
    await setMlflowRegisteredModelTag(formData.modelName, 'catalog.source', 'kf-model-catalog');
    await setMlflowRegisteredModelTag(formData.modelName, 'catalog.registered_by', author);
    if (formData.sourceModelFormat) {
      await setMlflowRegisteredModelTag(
        formData.modelName,
        'model_format',
        formData.sourceModelFormat,
      );
    }
  } catch (e) {
    if (e instanceof Error) {
      errors.registeredModel = e;
    }
    return { registeredModel, modelVersion, runId, errors };
  }

  // Step 3: Create the model version (equivalent to KF ModelVersion + ModelArtifact)
  try {
    const versionTags = customPropertiesToTags(formData.versionCustomProperties);
    modelVersion = await createMlflowModelVersion(
      formData.modelName,
      modelUri,
      formData.versionDescription,
      versionTags,
      runId,
    );

    // Set additional version tags
    if (formData.sourceModelFormat) {
      await setMlflowModelVersionTag(
        formData.modelName,
        modelVersion.version,
        'model_format',
        formData.sourceModelFormat,
      );
    }
    if (formData.sourceModelFormatVersion) {
      await setMlflowModelVersionTag(
        formData.modelName,
        modelVersion.version,
        'model_format_version',
        formData.sourceModelFormatVersion,
      );
    }
    await setMlflowModelVersionTag(
      formData.modelName,
      modelVersion.version,
      'catalog.source',
      'kf-model-catalog',
    );
  } catch (e) {
    if (e instanceof Error) {
      errors.modelVersion = e;
    }
  }

  return { registeredModel, modelVersion, runId, errors };
}
