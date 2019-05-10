import sqlalchemy as sa
import sqlalchemy_utils as sau
from enum import Enum
from srht.database import Base

class PatchsetStatus(Enum):
    proposed = "proposed"
    needs_revision = "needs_revision"
    superseded = "superseded"
    approved = "approved"
    rejected = "rejected"
    applied = "applied"

class Patchset(Base):
    __tablename__ = 'patchset'
    id = sa.Column(sa.Integer, primary_key=True)
    created = sa.Column(sa.DateTime, nullable=False)
    updated = sa.Column(sa.DateTime, nullable=False)
    subject = sa.Column(sa.Unicode(2048), nullable=False)
    prefix = sa.Column(sa.Unicode)
    version = sa.Column(sa.Integer, nullable=False)

    status = sa.Column(sau.ChoiceType(PatchsetStatus, impl=sa.String()),
            nullable=False, server_default="proposed")

    list_id = sa.Column(sa.Integer, sa.ForeignKey('list.id'))
    list = sa.orm.relationship('List', backref=sa.orm.backref('patchsets'))

    cover_letter_id = sa.Column(sa.Integer, sa.ForeignKey('email.id'))
    cover_letter = sa.orm.relationship("Email", foreign_keys=[cover_letter_id])

    superseded_by_id = sa.Column(sa.Integer, sa.ForeignKey('patchset.id'))
    superseded_by = sa.orm.relationship('Patchset',
            backref=sa.orm.backref('previous_version', remote_side=[id]),
            foreign_keys=[superseded_by_id])
