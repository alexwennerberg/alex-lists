#!/usr/bin/env python3
from distutils.core import setup
import subprocess
import os
import sys
import importlib.resources

with importlib.resources.path('srht', 'Makefile') as f:
    srht_path = f.parent.as_posix()

make = os.environ.get("MAKE", "make")
subp = subprocess.run([make, "SRHT_PATH=" + srht_path])
if subp.returncode != 0:
    sys.exit(subp.returncode)

ver = os.environ.get("PKGVER") or subprocess.run(['git', 'describe', '--tags'],
      stdout=subprocess.PIPE).stdout.decode().strip()

setup(
  name = 'listssrht',
  packages = [
      'listssrht',
      'listssrht.types',
      'listssrht.blueprints',
      'listssrht.blueprints.api',
      'listssrht.alembic',
      'listssrht.alembic.versions'
  ],
  version = ver,
  description = 'lists.sr.ht website',
  author = 'Drew DeVault',
  author_email = 'sir@cmpwn.com',
  url = 'https://git.sr.ht/~sircmpwn/lists.sr.ht',
  install_requires = [
      'srht',
      'emailthreads',
      'aiosmtpd',
      'asyncpg',
      'pygit2',
  ],
  license = 'AGPL-3.0',
  package_data={
      'listssrht': [
          'templates/*.html',
          'static/icons/*',
          'static/*',
          'schema.graphqls',
          'default_query.graphql',
      ]
  },
  scripts = [
      'listssrht-initdb',
      'listssrht-lmtp',
      'listssrht-migrate',
  ],
)
