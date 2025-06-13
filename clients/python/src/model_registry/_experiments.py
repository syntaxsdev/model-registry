from typing import Any

from model_registry.types.artifacts import ExperimentRunArtifact
from model_registry.types.experiments import ExperimentRun


class RunInfo:
    def __init__(
        self,
        run_id: str,
        experiment_id: str,
        user_id: str,
        status: str,
        start_time: str,
        end_time: str,
        stage: str,
        run_name: str,
    ):
        self.run_id = run_id
        self.experiment_id = experiment_id
        self.user_id = user_id
        self.start_time = start_time
        self.end_time = end_time
        self.status = status
        self.stage = stage
        self.run_name = run_name


class Run:
    def __init__(self):
        pass


class ActiveExperimentRun(Run):
    def __init__(self, experiment_run: ExperimentRun):
        Run.__init__(self)
        self.__exp_run = experiment_run
        # temporary solution
        self._logs: dict[str, dict[str, ExperimentRunArtifact]] = {
            "params": {},
            "metrics": {},
        }

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        pass

    def log_param(self, key: str, value: Any):
        # Temporary solution
        self._logs["params"][key] = ExperimentRunArtifact(
            name=key,
            uri=f"params/{key}",
            custom_properties={"value": value},
        )
    def log_metric(self, key: str, value: Any):
        # Temporary solution
        self._logs["metrics"][key] = ExperimentRunArtifact(
            name=key,
            uri=f"metrics/{key}",
            custom_properties={"value": value},
        )

    def get_logs(self) -> list[ExperimentRunArtifact]:
        logs: list[ExperimentRunArtifact] = []
        for inner_logs in self._logs.values():
            logs.extend(list(inner_logs.values()))
        return logs


class RunStack:
    def __init__(self):
        self.stack = []

    def push(self, run: Run):
        self.stack.append(run)

    def pop(self):
        return self.stack.pop()
