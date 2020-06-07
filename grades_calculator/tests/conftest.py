import json

# Third-party library imports
import pytest

# Local imports
from ..grades import Grades


@pytest.fixture(scope="module")
def grades():
    return Grades()
