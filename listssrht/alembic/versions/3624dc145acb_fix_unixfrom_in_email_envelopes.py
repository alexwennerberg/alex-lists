"""Fix unixfrom in email envelopes

Revision ID: 3624dc145acb
Revises: e5ae6528a8d5
Create Date: 2021-07-21 12:36:15.362297

"""

# revision identifiers, used by Alembic.
revision = '3624dc145acb'
down_revision = 'e5ae6528a8d5'

from alembic import op
import sqlalchemy as sa
import email
import email.policy
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import sessionmaker, Session as BaseSession, relationship

policy = email.policy.SMTPUTF8.clone(max_line_length=998)

Session = sessionmaker()
Base = declarative_base()

class Email(Base):
    __tablename__ = 'email'
    id = sa.Column(sa.Integer, primary_key=True)
    envelope = sa.Column(sa.Unicode, nullable=False)

def upgrade():
    bind = op.get_bind()
    session = Session(bind=bind)
    for record in session.query(Email).filter(Email.envelope.like("From %")).all():
        mail = email.message_from_string(record.envelope, policy=policy)
        record.envelope = mail.as_string()
    session.commit()


def downgrade():
    pass
