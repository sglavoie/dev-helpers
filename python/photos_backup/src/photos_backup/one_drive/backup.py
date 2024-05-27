from office365.graph_client import GraphClient


class Backup:
    def __init__(self, dry_run: bool) -> None:
        self.dry_run = dry_run
        self._client = None
        self._drive = None

        if not dry_run:
            from photos_backup.one_drive.auth import acquire_token_func

            self._client = GraphClient(acquire_token_func)
            self._drive = self._client.me.drive

        # TODO

    def backup(self) -> None: ...

    def _set_up_paths(self) -> None: ...
