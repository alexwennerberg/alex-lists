"""Expand size of subject & header columns

Revision ID: 803817bfd568
Revises: c3c2bea8d735
Create Date: 2019-03-06 08:52:13.408717

"""

# revision identifiers, used by Alembic.
revision = '803817bfd568'
down_revision = 'c3c2bea8d735'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.alter_column("email", "subject", type_=sa.Unicode(2048))
    op.alter_column("email", "message_id", type_=sa.Unicode(2048))


def downgrade():
    op.alter_column("email", "subject", type_=sa.Unicode(512))
    op.alter_column("email", "message_id", type_=sa.Unicode(512))
