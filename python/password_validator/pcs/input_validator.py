"""
This class serves to receive an input which is to be used as a string by the
PasswordValidator class. Its purpose is to maintain flexibility in how the
input can be processed and tested separately prior to being used by
PasswordValidator.
"""
from typing import NoReturn


class InputValidator:
    """
    Reads in a string input.
    Rejects empty strings as well as any input that is not a string.
    """

    def __init__(self, input_string: str) -> None:
        self.input = input_string
        self.check_for_input_validation_errors()

    def check_for_input_validation_errors(self):
        self._raise_value_error_on_empty_input()
        self._raise_type_error_on_input_not_being_a_string()

    def _raise_value_error_on_empty_input(self) -> NoReturn:
        if self.input == "":
            raise ValueError("Input cannot be empty...")

    def _raise_type_error_on_input_not_being_a_string(self) -> NoReturn:
        if not isinstance(self.input, str):
            raise TypeError("Input must be a string...")
