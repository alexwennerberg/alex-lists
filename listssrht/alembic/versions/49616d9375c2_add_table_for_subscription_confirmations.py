"""Add table for subscription confirmations

Revision ID: 49616d9375c2
Revises: 2a4319aa8d00
Create Date: 2024-04-23 10:40:58.084134

"""

# revision identifiers, used by Alembic.
revision = '49616d9375c2'
down_revision = '2a4319aa8d00'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.execute("""
    CREATE TABLE subscription_request (
        id serial PRIMARY KEY,
        email CHARACTER VARYING(512),
        confirmation_hash CHARACTER VARYING(128),
        list_id integer NOT NULL references "list"(id)
    );
    """)


def downgrade():
    op.execute("""
    DROP TABLE subscription_request;
    """)
