# Standard library imports
from typing import Callable

# Local imports
from pcs.input_validator import InputValidator
from pcs.password_validator import (
    PasswordValidator,
    PasswordValidatorAdminUser,
    PasswordValidatorRegularUser,
)


def select_user_type() -> str:
    """
    Prompt the user to select a type of user to validate the password
    strength against the correct set of requirements.

    Returns:
        str: String identifying a user type for which we will validate the
             password strength.
    """
    print("Please enter the user type to validate the correct set of")
    print("password strength requirements.")

    print("User type    Number")
    print("-------------------")
    print("Regular           1")
    print("Admin             2")

    msg_prompt = "Please type the relevant user type number"
    valid_choices = {"1": "regular", "2": "admin"}
    user_type = loop_until_choice_is_correct(msg_prompt, valid_choices)
    return user_type


def loop_until_choice_is_correct(msg_prompt: str, valid_choices: dict) -> str:
    """
    Loop until a valid value is provided, which must match a key from
    `valid_choices`.

    Args:
        msg_prompt (str): Prompt to display to the user to enter a choice.
        valid_choices (dict, optional): Keys are valid choices.

    Returns:
        str: String corresponding to the value of the matching key in
        `valid_choice`.
    """
    choice = None
    while True:
        input_string = input(f"{msg_prompt}: ")
        if input_string not in valid_choices:
            print("Please enter a valid choice.")
        else:
            choice = valid_choices[input_string]
            break

    return choice


def execute_appropriate_validator(
    user_type: str, password: str
) -> Callable[[str], Callable]:
    """
    Return a class instance of the correct password validator to use based on
    the type of user.

    Args:
        user_type (str): Identify the type of user.
        password (str): Password to validate for the given type of user.

    Returns:
        [type]: Instance of the expected password validator subclass to use
                customized based on the user type.
    """
    if user_type == "regular":
        return PasswordValidatorRegularUser(password)
    if user_type == "admin":
        return PasswordValidatorAdminUser(password)
    return PasswordValidator


def main_entry_point() -> None:
    """
    Main entry point to the program.
    """
    user_type = select_user_type()
    input_string = input(
        f"Please enter a password for the {user_type.capitalize()} user: "
    )
    password = InputValidator(input_string).input
    password_validator = execute_appropriate_validator(user_type, password)

    if password_validator.is_valid():
        print("Password ACCEPTED.")
    else:
        print("Password REJECTED.")
