import json

import pytest

from model_registry import ModelRegistry


@pytest.mark.e2e
def test_start_experiment_run(client: ModelRegistry):
    with client.start_experiment_run(experiment_name="Experiment_Test") as run:
        schema = {"epochs": {}}
        schema_json = json.dumps(schema)
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
            schema=f"{str(schema_json)}",
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
def test_start_experiment_run_with_advanced_scenarios(client: ModelRegistry):
    with client.start_experiment_run(experiment_name="Experiment_Test") as run:
        run.log_param("input1", 5.75)
        run.log_param("input1", 500)
        for i in range(10):
            run.log_metric(f"metric_{i}", value=i * 1000, step=i, timestamp="0")

    assert len(run.get_logs()) == 11
    assert run.get_log("params", "input1").value == 500


@pytest.mark.e2e
def test_experiments(client: ModelRegistry):
    with client.start_experiment_run(experiment_name="Experiment_Test_2") as run:
        pass
    found_exp = False
    for experiment in client.get_experiments():
        if experiment.name == "Experiment_Test_2":
            found_exp = True
    assert found_exp


@pytest.mark.e2e
def test_get_experiment_runs(client: ModelRegistry):
    with client.start_experiment_run(experiment_name="Experiment_Test_2") as run:
        pass
    runs = client.get_experiment_runs(experiment_name="Experiment_Test_2")
    found_exp_run_by_id = False
    found_exp_run_by_name = False
    for r in runs:
        if r.id == run.info.id:
            found_exp_run_by_id = True
        if r.name == run.info.name:
            found_exp_run_by_name = True
    assert found_exp_run_by_id
    assert found_exp_run_by_name
