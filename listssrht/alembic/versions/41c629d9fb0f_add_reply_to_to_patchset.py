"""Add reply_to to patchset

Revision ID: 41c629d9fb0f
Revises: 98bb95b72e3a
Create Date: 2020-07-14 09:01:24.032627

"""

# revision identifiers, used by Alembic.
revision = '41c629d9fb0f'
down_revision = '98bb95b72e3a'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.add_column('patchset', sa.Column('reply_to', sa.Unicode))


def downgrade():
    op.drop_column('patchset', 'reply_to')
