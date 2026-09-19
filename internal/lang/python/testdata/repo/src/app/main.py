# pyright: basic
from __future__ import annotations

import json as j  # noqa: F401
import os
import os.path  # noqa: F401
from typing import List  # noqa: F401, UP035

import black  # type: ignore # noqa: F401
import certifi  # type: ignore # noqa: F401
import notdeclared.sub  # type: ignore # noqa: F401
import numpy as np  # type: ignore # noqa: F401
import pytest  # type: ignore # noqa: F401
import requests  # type: ignore # noqa: F401
import yaml  # type: ignore # noqa: F401
from bs4 import BeautifulSoup  # type: ignore # noqa: F401

from app.helpers import thing  # noqa: F401
from app.models import *  # type: ignore

from ..outside import x  # type: ignore # noqa: F401
from . import utils  # type: ignore # noqa: F401
from .models import User  # type: ignore # noqa: F401
from .models.user import User as U  # type: ignore # noqa: F401


def run():
    pass


@decorator  # type: ignore
def dec():
    pass


class Service:
    def start(self):
        pass

    @property
    def name(self):
        return "svc"


CONSTANT = 1
