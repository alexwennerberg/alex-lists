"""Add unique constraint to lists

Revision ID: 3e671a75a477
Revises: 3624dc145acb
Create Date: 2021-08-26 14:42:51.478577

"""

# revision identifiers, used by Alembic.
revision = '3e671a75a477'
down_revision = '3624dc145acb'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.execute("""
    ALTER TABLE list
    ADD CONSTRAINT uq_list_owner_id_name
    UNIQUE (owner_id, name);
    """)


def downgrade():
    op.execute("""
    ALTER TABLE list
    DROP CONSTRIANT uq_list_owner_id_name;
    """)
