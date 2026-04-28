import os
import pytest
import pymongo
import yaml

_default_config = os.path.join(os.path.dirname(__file__), "config.yaml")
CONFIG_PATH = os.environ.get("INTEGRATION_CONFIG") or _default_config


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


@pytest.fixture(scope="session")
def mongo_client(mongo_url):
    client = pymongo.MongoClient(mongo_url, directConnection=True)
    yield client
    client.close()
