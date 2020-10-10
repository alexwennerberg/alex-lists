"""Add unique constraint on (message_id, list_id)

Revision ID: e5ae6528a8d5
Revises: ed291e59b2a0
Create Date: 2020-10-10 13:13:57.354053

"""

# revision identifiers, used by Alembic.
revision = 'e5ae6528a8d5'
down_revision = 'ed291e59b2a0'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.create_unique_constraint('uq_email_list_message_id', 'email',
            ['list_id', 'message_id'])


def downgrade():
    op.drop_constraint('uq_email_list_message_id')
