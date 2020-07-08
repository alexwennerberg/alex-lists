import sqlalchemy as sa
import sqlalchemy_utils as sau
from enum import Enum
from srht.database import Base

class RepositoryType(Enum):
    gitsrht = "gitsrht"
    hgsrht = "hgsrht"
    external_git = "external_git"
    external_hg = "external_hg"

class ListRepository(Base):
    __tablename__ = 'list_repository'
    id = sa.Column(sa.Integer, primary_key=True)
    created = sa.Column(sa.DateTime, nullable=False)
    updated = sa.Column(sa.DateTime, nullable=False)
    name = sa.Column(sa.String(128), nullable=False)
    upstream = sa.Column(sa.String(2048), nullable=False)
    repo_type = sa.Column(sau.ChoiceType(impl=sa.String), nullable=False)

    mailing_list_id = sa.Column(sa.Integer,
            sa.ForeignKey('list.id'), nullable=False)
    mailing_list = sa.orm.relationship('List', backref=sa.orm.backref('repos'))

    def __repr__(self):
        return '<ListRepository {} {}>'.format(self.id, self.name)

    def to_dict(self, short=False):
        return {
            "name": self.name,
            "upstream": self.upstream,
            **({
                "created": self.created,
                "updated": self.updated,
                "type": self.repo_type.value,
            } if not short else {})
        }
