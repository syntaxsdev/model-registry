from contextlib import AbstractContextManager
from dataclasses import dataclass, field
from typing import Any, Callable

from model_registry.core import ModelRegistryAPIClient
from model_registry.types.artifacts import (
    ExperimentRunArtifact,
    ExperimentRunArtifactTypes,
)
from model_registry.types.experiments import ExperimentRun

# @dataclass
# class ExperimentRunContext:
#     experiment_run: ExperimentRun
#     api: ModelRegistryAPIClient
#     async_runner: Callable


class ActiveExperimentRun(AbstractContextManager):
    def __init__(
        self,
        experiment_run: ExperimentRun,
        api: ModelRegistryAPIClient,
        async_runner: Callable,
    ):
        self.__exp_run = experiment_run
        self.__api = api
        self.__async_runner = async_runner
        # temporary solution
        self._logs: ExperimentRunArtifactTypes = ExperimentRunArtifactTypes()

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        """Exit the context manager and upsert the logs to the experiment run."""
        temp_artifacts: ExperimentRunArtifactTypes = ExperimentRunArtifactTypes()
        for log in self.get_logs():
            server_log = self.__async_runner(
                self.__api.upsert_experiment_run_artifact(
                    experiment_run_id=self.__exp_run.id, artifact=log
                )
            )
            era = ExperimentRunArtifact.from_doc(server_log)
            temp_artifacts.params[era.name] = era
        self._logs = temp_artifacts

    def log_param(self, key: str, value: Any):
        """Log a parameter to the experiment run."""
        self._logs.params[key] = ExperimentRunArtifact(
            name=key,
            uri="",
            custom_properties={key: value},
        )

    def log_metric(self, key: str, value: Any):
        """Log a metric to the experiment run."""
        self._logs.metrics[key] = ExperimentRunArtifact(
            name=key,
            uri="",
            custom_properties={key: value},
        )

    def get_log(self, type: str, key: str) -> ExperimentRunArtifact:
        """Get a log from the experiment run.

        Args:
            type: Type of the log.
            key: Key of the log.
        """
        return self._logs.__getattribute__(type)[key]

    def get_logs(self) -> list[ExperimentRunArtifact]:
        """Return every recorded artifact (params + metrics) in one flat list."""
        params = self._logs.params.values()
        metrics = self._logs.metrics.values()

        return list(params) + list(metrics)
