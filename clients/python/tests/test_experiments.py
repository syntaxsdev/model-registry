import json

import pytest

from model_registry import ModelRegistry, utils


@pytest.fixture
def schema_json():
    schema = {"epochs": {}}
    return json.dumps(schema)


@pytest.mark.e2e
def test_start_experiment_run(client: ModelRegistry, schema_json: str):
    with client.start_experiment_run(experiment_name="Experiment_Test") as run:
        run.log_param("input1", 5.75)
        run.log_metric(
            key="rval",
            value=10,
            step=4,
            timestamp="0",
            description="This is a test metric",
        )
        run.log_dataset(
            name="dataset_1",
            source_type="local",
            uri="s3://datasets/test",
            schema=schema_json,
            profile="random_profile",
        )

    assert len(run.get_logs()) == 3
    param = run.get_log("params", "input1")
    metric = run.get_log("metrics", "rval")
    dataset = run.get_log("datasets", "dataset_1")
    assert param
    assert metric
    assert dataset

    assert param.value == 5.75
    assert metric.value == 10
    assert metric.step == 4
    assert metric.timestamp == "0"
    assert metric.description == "This is a test metric"
    assert metric.name == "rval"


@pytest.mark.e2e
def test_start_experiment_run_with_advanced_scenarios(
    client: ModelRegistry, get_temp_dir_with_models, patch_s3_env, schema_json: str
):
    with client.start_experiment_run(experiment_name="Experiment_Test") as run:
        run.log_param("input1", 5.75)
        run.log_param("input1", 500)
        for i in range(10):
            run.log_metric(f"metric_{i}", value=i * 1000, step=i, timestamp="0")

    assert len(run.get_logs()) == 11
    assert run.get_log("params", "input1").value == 500

    with client.start_experiment_run(
        experiment_name="Experiment_Test_URI_Provided"
    ) as run:
        run.log_dataset(
            name="dataset_1",
            source_type="s3",
            uri="s3://datasets/test",
            schema=schema_json,
            profile="random_profile",
        )
    assert run.get_log("datasets", "dataset_1").uri == "s3://datasets/test"

    # Test actual
    model_dir, _ = get_temp_dir_with_models
    bucket, s3_endpoint, access_key_id, secret_access_key, region = patch_s3_env
    with client.start_experiment_run(experiment_name="Experiment_Test_3") as run:
        run.log_dataset(
            name="dataset_1",
            source_type="local",
            schema=schema_json,
            profile="random_profile",
            file_path=model_dir,
            s3_auth=utils.S3Params(
                endpoint_url=s3_endpoint,
                bucket_name=bucket,
                s3_prefix="datasets",
                access_key_id=access_key_id,
                secret_access_key=secret_access_key,
                region=region,
            ),
        )
    assert run.get_log("datasets", "dataset_1").uri.startswith("s3://")


@pytest.mark.e2e
def test_experiments(client: ModelRegistry):
    with client.start_experiment_run(experiment_name="Experiment_Test_2"):
        pass
    found_exp = False
    for experiment in client.get_experiments():
        if experiment.name == "Experiment_Test_2":
            found_exp = True
    assert found_exp


@pytest.mark.e2e
def test_get_experiment_runs(client: ModelRegistry):
    """This tests:
    - get_experiment_runs(name)
    - get_experiment_runs(id)
    """
    with client.start_experiment_run(experiment_name="Experiment_Test_3") as run:
        run.log_param("input1", 5.75)
    runs_by_name = client.get_experiment_runs(experiment_name="Experiment_Test_3")
    runs_by_id = client.get_experiment_runs(experiment_id=run.info.experiment_id)

    assert runs_by_name.next_item().id == runs_by_id.next_item().id
    runs_by_name.restart()
    runs_by_id.restart()

    found_exp_run_by_id = False
    found_exp_run_by_name = False

    for r in runs_by_name:
        if r.name == run.info.name:
            found_exp_run_by_name = True

    for r in runs_by_id:
        if r.id == run.info.id:
            found_exp_run_by_id = True

    assert found_exp_run_by_id
    assert found_exp_run_by_name


@pytest.mark.e2e
def test_get_experiment_run_with_artifact_types(
    client: ModelRegistry, schema_json: str
):
    with client.start_experiment_run(experiment_name="Experiment_Test_4") as run:
        run.log_dataset(
            name="dataset_1",
            source_type="local",
            uri="s3://datasets/test",
            schema=schema_json,
            profile="random_profile",
            description="This is a test dataset",
        )
        run.log_metric(
            key="metric_1",
            value=10,
            step=4,
            timestamp="0",
            description="This is a test metric",
        )
        run.log_param(
            key="param_1",
            value=10,
            description="This is a test param",
        )

    dataset_log = client.get_experiment_run_logs(
        run_id=run.info.id,
    )
    assert dataset_log.next_item().name.endswith("1")
    assert dataset_log.next_item()
    assert dataset_log.next_item()
    try:
        # fail if we get a 4th item
        dataset_log.next_item()
        pytest.fail("Expected StopIteration")
    except StopIteration:
        assert True
