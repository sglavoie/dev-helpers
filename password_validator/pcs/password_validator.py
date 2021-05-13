"""
This class takes in a string and validates the strength of a password.

It prints a message telling the user whether a password meets the strength
requirements or not.
"""
# Standard library imports
from abc import ABC
from string import ascii_letters, digits
from typing import Callable


def extract_class_methods_validators(class_: Callable) -> list:
    """
    Return a list of the class methods available in the class calling
    this function to get a list of methods that are used to validate
    password strength requirements.

    Returns:
        list: Contains the name (str) of all methods starting with
        `_has_`.
    """
    # limit scope of added functions to the custom ones: _has_...()
    return [method for method in dir(class_) if method.startswith("_has_") is True]


def decorate_validation_errors(func: Callable) -> Callable:
    """
    Decorator that wraps the output of `func` with separators if validation
    errors are found.
    """

    def wrapper(*args, **kwargs):
        # All conditions were met: let's not print anything around the output
        if all(value is None for value in args[0]):
            return

        print("=" * 60)
        func(*args, **kwargs)
        print("=" * 60)

    return wrapper


class PasswordValidator(ABC):
    """
    This abstract class is responsible for validating the strength of a
    password given as an argument for a specific type of user (either
    Admin or Regular). Calling its `is_valid()` method will return a
    Boolean value telling the user whether a password meets all the
    necessary requirements or not.
    """

    password: str  # password to validate
    min_length: int  # min. password length required
    min_letters: int  # min. number of letters required
    min_numbers: int  # min. number of digits required
    condition_methods: list  # list of conditions applied by subclasses

    def is_valid(self) -> bool:
        """
        Execute all the methods starting with an underscore, store their
        Boolean result. If all the conditions are met, return True.
        Otherwise, return False.
        """
        conditions_are_met = []
        reasons = []
        for method in self.condition_methods:
            class_method = getattr(self, method)
            condition_met, reason = class_method()
            conditions_are_met.append(condition_met)
            reasons.append(reason)

        self.print_unmet_conditions(reasons)
        return all(conditions_are_met)

    @staticmethod
    @decorate_validation_errors
    def print_unmet_conditions(reasons: list) -> None:
        for reason in reasons:
            if reason is not None:
                print(reason)

    def _has_valid_length(self) -> tuple:
        valid_length = len(self.password) >= self.min_length
        invalid_reason = None

        if not valid_length:
            plural = "" if self.min_length <= 1 else "s"
            invalid_reason = (
                f"Password must be at least {self.min_length} "
                f"character{plural} in length."
            )
        return (valid_length, invalid_reason)

    def _has_enough_letters(self) -> tuple:
        count = 0
        invalid_reason = None
        for character in self.password:
            if character in ascii_letters:
                count += 1
        valid_count = count >= self.min_letters

        if not valid_count:
            plural = "" if self.min_letters <= 1 else "s"
            invalid_reason = (
                f"Password must have at least {self.min_letters}" f" letter{plural}."
            )
        return (valid_count, invalid_reason)

    def _has_enough_numbers(self) -> tuple:
        count = 0
        invalid_reason = None
        for character in self.password:
            if character in digits:
                count += 1
        valid_count = count >= self.min_numbers

        if not valid_count:
            plural = "" if self.min_numbers <= 1 else "s"
            invalid_reason = (
                f"Password must have at least {self.min_numbers}" f" number{plural}."
            )
        return (valid_count, invalid_reason)


class PasswordValidatorRegularUser(PasswordValidator):
    def __init__(self, password: str) -> None:
        self.password = password
        self.min_length = 10
        self.min_letters = 1
        self.min_numbers = 1
        self.condition_methods = extract_class_methods_validators(
            PasswordValidatorRegularUser
        )
        super().__init__()


class PasswordValidatorAdminUser(PasswordValidator):
    def __init__(self, password: str) -> None:
        self.password = password
        self.min_length = 13
        self.min_letters = 1
        self.min_numbers = 1
        self.min_special_char = 3
        self.special_chars = "!@#$%^&*"  # list of accepted special characters
        self.condition_methods = extract_class_methods_validators(
            PasswordValidatorAdminUser
        )
        super().__init__()

    def _has_enough_special_characters(self) -> tuple:
        count = 0
        invalid_reason = None
        for character in self.password:
            if character in self.special_chars:
                count += 1
        valid_count = count >= self.min_special_char
        if not valid_count:
            plural = "" if self.min_special_char <= 1 else "s"
            invalid_reason = (
                f"Password must have at least {self.min_special_char}"
                f" special character{plural}."
            )
        return (valid_count, invalid_reason)
