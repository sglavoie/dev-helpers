# Have I been Pwned?

Wonder no more! This simple script will communicate with [Have I Been Pwned API](https://haveibeenpwned.com/API/v2) and tell you the truth... At least the known truth.

---

# Requirements

You will need [Click](https://github.com/pallets/click) and [requests](https://github.com/kennethreitz/requests) for this script to work:

    pip install click requests

---

# Usage

Execute this script in a terminal with Python3.6+ in one of the following ways:

    $ python3 pwned.py -p MyPassword
    $ python3 pwned.py --password MyPassword
    $ python3 pwned.py -p 'MyPassword'
    $ python3 pwned.py -p "MyPassword"
    $ python3 pwned.py -p "MyPasswordWith\"Quotes\"Inside"
