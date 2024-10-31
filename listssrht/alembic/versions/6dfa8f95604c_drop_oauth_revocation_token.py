"""Drop oauth_revocation_token

Revision ID: 6dfa8f95604c
Revises: 49616d9375c2
Create Date: 2024-10-31 12:59:18.888422

"""

# revision identifiers, used by Alembic.
revision = '6dfa8f95604c'
down_revision = '49616d9375c2'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.execute("""
    ALTER TABLE "user" DROP COLUMN oauth_revocation_token;
    """)


def downgrade():
    op.execute("""
    ALTER TABLE "user"
    ADD COLUMN oauth_revocation_token character varying(256);
    """)
