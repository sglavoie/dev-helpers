# Have I been Pwned?

Wonder no more! This simple script will communicate with [Have I Been Pwned API](https://haveibeenpwned.com/API/v2) and tell you the truth... At least the known truth.

---

# Requirements

You will need [Click](https://github.com/pallets/click) and [requests](https://github.com/kennethreitz/requests) for this script to work:

    pip install click requests

or from this directory:

    pip install -r requirements.txt

---

# Usage

Execute this script in a terminal with Python3.6+ in one of the following ways:

    python pwned.py  # for general help
    python pwned.py check --help  # same as previous line
    python pwned.py check -p MyPassword
    python pwned.py check --password MyPassword
    python pwned.py check -p 'MyPassword'
    python pwned.py check -p "MyPassword"

For more complicated passwords, you have to use quotes and
escape symbols with \ where appropriate:

    python pwned.py check -p "as0d9\"asg0''A=)SYD"
