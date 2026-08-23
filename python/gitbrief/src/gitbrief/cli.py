"""Entry point: the root command group and the subcommands attached to it."""

import click

from gitbrief.commands.completion import install_completion
from gitbrief.commands.doctor import doctor
from gitbrief.commands.groups import group_group
from gitbrief.commands.history import history_group
from gitbrief.commands.projects import add, list_projects, remove
from gitbrief.commands.scan import scan
from gitbrief.commands.settings import config_group
from gitbrief.commands.summary import summary
from gitbrief.commands.templates import template_group


@click.group()
@click.version_option(package_name="gitbrief")
def cli() -> None:
    """AI-powered git activity summarizer."""


for _command in (
    add,
    remove,
    list_projects,
    config_group,
    group_group,
    doctor,
    install_completion,
    summary,
    template_group,
    history_group,
    scan,
):
    cli.add_command(_command)


if __name__ == "__main__":
    cli()
