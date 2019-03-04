"""Add oauthtoken table

Revision ID: 84d51b97a028
Revises: a03b9606f3c6
Create Date: 2019-03-04 09:37:18.875852

"""

# revision identifiers, used by Alembic.
revision = '84d51b97a028'
down_revision = 'a03b9606f3c6'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.create_table('oauthtoken',
        sa.Column("id", sa.Integer, primary_key=True),
        sa.Column("created", sa.DateTime, nullable=False),
        sa.Column("updated", sa.DateTime, nullable=False),
        sa.Column("expires", sa.DateTime, nullable=False),
        sa.Column("token_hash", sa.String(128), nullable=False),
        sa.Column("token_partial", sa.String(8), nullable=False),
        sa.Column("scopes", sa.String(512), nullable=False),
        sa.Column("user_id", sa.Integer, sa.ForeignKey('user.id')))


def downgrade():
    op.drop_table('oauthtoken')
