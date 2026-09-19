from setuptools import setup

setup(
    install_requires=["rich==13.7.0", 'attrs'],
    extras_require={"docs": ["sphinx>=7"], "lint": ["ruff"]},
)
