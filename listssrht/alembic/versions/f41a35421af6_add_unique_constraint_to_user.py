"""Add unique constraint to user

Revision ID: f41a35421af6
Revises: 9e003f1683b8
Create Date: 2025-01-06 10:34:45.047768

"""

# revision identifiers, used by Alembic.
revision = 'f41a35421af6'
down_revision = '9e003f1683b8'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.execute("""
    ALTER TABLE "user"
    ADD CONSTRAINT user_email_key
    UNIQUE (email);
    """)


def downgrade():
    op.execute("""
    ALTER TABLE "user"
    DROP CONSTRIANT user_email_key
    """)
