"""Add uniqe constraints to access

Revision ID: 05c33e92b0e1
Revises: 3e671a75a477
Create Date: 2021-09-07 10:14:45.427123

"""

# revision identifiers, used by Alembic.
revision = '05c33e92b0e1'
down_revision = '3e671a75a477'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.execute("""
    ALTER TABLE access
        ADD CONSTRAINT uq_access_list_id_user_id
        UNIQUE (list_id, user_id);

    ALTER TABLE access
        ADD CONSTRAINT uq_access_list_id_email
        UNIQUE (list_id, email);
    """)


def downgrade():
    op.execute("""
    ALTER TABLE access DROP CONSTRIANT uq_access_list_id_user_id;
    ALTER TABLE access DROP CONSTRIANT uq_access_list_id_email;
    """)
