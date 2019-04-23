"""
Simple script that will take advantage of haveibeenpwned.com's API to
find out if your password has been breached.
"""
# Standard library imports
import hashlib

# Third party library imports
import click
import requests


# Needed to access API
PASS_URL = 'https://api.pwnedpasswords.com/range/'

# Script information
VERSION = '0.1.1'
LOGO = r'''
 __                        __   __
|  |--.---.-.--.--.-----. |__| |  |--.-----.-----.-----.
|     |  _  |  |  |  -__| |  | |  _  |  -__|  -__|     |
|__|__|___._|\___/|_____| |__| |_____|_____|_____|__|__|

                               __  _____
.-----.--.--.--.-----.-----.--|  ||__   |
|  _  |  |  |  |     |  -__|  _  |',  ,-'
|   __|________|__|__|_____|_____| |--|
|__|                               '--'  '''


def print_info(context, param, value):  # `param` required by `click`
    """Print information about the program and exit."""
    if not value or context.resilient_parsing:
        return
    click.secho(f'{LOGO}\n{"Python 3.6+ Checker".rjust(60)}\n'
                f'{("v" + VERSION).rjust(60)}', fg='blue', bold=True)
    context.exit()


def pass_to_sha1(password):
    """Will convert `password` with SHA-1 algorithm and return the short
    version with the first five characters and the long version with
    everything but the first five characters as a tuple."""

    my_password = password.encode()  # converts to byte string

    complete_hash = hashlib.sha1(my_password).hexdigest().upper()
    long_hash = complete_hash[5:]
    short_hash = complete_hash[:5]

    return short_hash, long_hash


def check_password(short_hash, long_hash):
    """Consult haveibeenpwned.com to see how many times the password has
    been breached."""
    click.secho('Please note that only the 5 first characters from your '
                'SHA-1 ENCRYPTED password are sent to haveibeenpwned.com.',
                fg='yellow', bold=True)
    response = requests.get(PASS_URL + short_hash)
    lines = response.text.split()
    for line in lines:
        if long_hash in line:
            num_times = 'occurrences' if int(line[36:]) > 1 else 'occurence'
            click.secho('Your password was found!', fg='red', bold=True)
            click.echo(f'→ {line[36:]} {num_times}')
            break
    else:
        click.secho('Your password was NOT found!', fg='green')


@click.group()
@click.option(
    '--info',
    is_flag=True,
    callback=print_info,
    expose_value=False,
    is_eager=True,
    help="Print some information about the program and exit.")
@click.version_option(version=VERSION, message="%(version)s")
def hibp():
    """A simple command-line interface to make use of
    haveibeenpwned.com's data.

    Type `python pwned.py usage` for more info (or whatever way you
    call Python 3)."""


@hibp.command()
def usage():
    """Give examples on how to use this script."""
    click.secho('Examples:', fg='blue')
    click.secho('python pwned.py check --help for more info.', fg='green')
    click.secho('python pwned.py check -p MyPasswordHere', fg='yellow')
    click.secho('python pwned.py check --password MyPasswordHere', fg='green')

    click.echo('')
    click.secho('For more complicated passwords, you have to use quotes and ',
                fg='blue')
    click.secho('escape symbols with \\ where appropriate:', fg='blue')
    click.secho('python pwned.py check -p "as0d9\\"asg0\'\'A=)SYD"',
                fg='green')


@hibp.command()
@click.option(
    '-p',
    '--password',
    default=None,
    help="Reveal if match exists with `password`.")
def check(password):
    """Tell if your password has been breached."""
    if password:
        short_hash, long_hash = pass_to_sha1(password)
        check_password(short_hash, long_hash)
    if not password:
        click.secho('Run `pwned.py check --help` for more info.', fg='red')


if __name__ == '__main__':
    hibp()
