from __future__ import annotations

import os
import os.path
import json as j
from typing import List

import requests
import yaml
from bs4 import BeautifulSoup
import numpy as np
import certifi
import pytest
import black
import notdeclared.sub

from . import utils
from .models import User
from .models.user import User as U
from ..outside import x
from app.helpers import thing
from app.models import *


def run():
    pass


@decorator
def dec():
    pass


class Service:
    def start(self):
        pass

    @property
    def name(self):
        return "svc"


CONSTANT = 1
