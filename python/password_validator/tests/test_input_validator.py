# Standard library imports
from decimal import Decimal
from typing import Any

# Third-party libraries
import pytest

# Local imports
from pcs.input_validator import InputValidator


def test_input_is_set_to_value_passed_as_argument():
    """
    The `input` class attribute of InputValidator should be set correctly.
    """
    input_string = "test_string"
    assert InputValidator(input_string).input == input_string


def test__raise_value_error_on_empty_input():
    """
    The tool should not accept an empty password.
    """
    with pytest.raises(ValueError):
        assert InputValidator("")


@pytest.mark.parametrize(
    "input_",
    [
        (None),
        (True),
        (2j),
        (Decimal(12)),
        (b"asd"),
        (12345),
        (["asd"]),
        (("asd",)),
        ({"asd": "123"}),
        ({}),
    ],
)
def test__raise_type_error_on_input_not_being_a_string(input_: Any):
    """
    The tool should not accept anything that is not a string.
    """
    with pytest.raises(TypeError):
        assert InputValidator(input_)
