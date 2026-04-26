import os
import pytest
import yaml

CONFIG_PATH = os.path.join(os.path.dirname(__file__), "config.yaml")


@pytest.fixture(scope="session")
def config():
    with open(CONFIG_PATH) as f:
        return yaml.safe_load(f)


@pytest.fixture(scope="session")
def gateway(config):
    return config["gateway_url"]


@pytest.fixture(scope="session")
def gatewaypriv(config):
    return config["gatewaypriv_url"]


@pytest.fixture(scope="session")
def subscriber(config):
    return config["subscriber_url"]


@pytest.fixture(scope="session")
def mongo_url(config):
    return config["mongo_url"]
