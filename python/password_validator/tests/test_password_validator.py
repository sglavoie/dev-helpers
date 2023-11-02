# Third-party libraries
import pytest

# Local imports
from pcs.input_validator import InputValidator
from pcs.password_validator import PasswordValidatorAdminUser as pv_admin
from pcs.password_validator import PasswordValidatorRegularUser as pv_regular


def test_password_validator_reads_input_correctly():
    """
    Make sure that PasswordValidator receives a string from InputValidator
    when it is non-empty.
    """
    input_string = "xc9v86"
    password = InputValidator(input_string).input
    assert pv_regular(password).password == input_string


@pytest.mark.parametrize(
    "password,expected_bool",
    [
        ("a", False),  # length: 1
        ("12345", False),  # length: 5
        ("abc12cv", False),  # length: 7
        ("abc12cvrt9", True),  # length: 10 (edge case)
        ("password123", True),  # length: 11
    ],
)
def test__has_valid_length_ie_at_least_10_characters(
    password: str, expected_bool: bool
):
    """
    _has_valid_length() returns True if the password length meets the
    requirement specified in the test name, False otherwise.
    """
    assert pv_regular(password)._has_valid_length()[0] == expected_bool


@pytest.mark.parametrize(
    "password,expected_bool",
    [
        ("a", True),  # 1 letter
        ("12345", False),  # 0 letter
        ("4545v", True),  # 1 letter (at the end)
        ("b4545", True),  # 1 letter (at the start)
        ("45c45", True),  # 1 letter (in the middle)
        ("a12v9", True),  # 2 letters
        ("password123", True),  # 8 letters
    ],
)
def test__has_enough_letters_ie_at_least_1(password: str, expected_bool: bool):
    """
    _has_enough_letters() returns True if the number of letters in the
    password meets the requirement specified in the test name, False
    otherwise.
    """
    assert pv_regular(password)._has_enough_letters()[0] == expected_bool


@pytest.mark.parametrize(
    "password,expected_bool",
    [
        ("a", False),  # 0 digit
        ("12345", True),  # 5 digits
        ("abc4", True),  # 1 digit (at the end)
        ("4abc", True),  # 1 digit (at the start)
        ("ab3cd", True),  # 1 digit (in the middle)
        ("a12vxckjh", True),  # 2 digits
        ("password123", True),  # 3 digits
    ],
)
def test__has_enough_numbers_ie_at_least_1(password: str, expected_bool: bool):
    """
    _has_enough_numbers() returns True if the number of digits in the
    password meets the requirement specified in the test name, False
    otherwise.
    """
    assert pv_regular(password)._has_enough_numbers()[0] == expected_bool


@pytest.mark.parametrize(
    "password,expected_bool",
    [
        ("asdlkfjsadlfjAH", False),  # 0 special character
        ("@dlkfjsadlfjAH", False),  # 1 special character, at the start
        ("asdlkf!jsadlf", False),  # 1 special character, in the middle
        ("asdlkfjsadlf&", False),  # 1 special character, at the end
        ("asd%lkfjsad$lf", False),  # 2 special characters
        ("asdl*kf^jsa#dlf", True),  # 3 special characters
        (r"a%sdl*kf^#dlf", True),  # 4 special characters
    ],
)
def test__has_enough_special_characters_ie_at_least_3(
    password: str, expected_bool: bool
):
    """
    _has_enough_special_characters() returns True if the number of
    special characters in the password meets the requirement specified
    in the test name, False otherwise.

    Accepted special characters:
    ! @ # $ % ^ & *
    """
    assert pv_admin(password)._has_enough_special_characters()[0] == expected_bool
