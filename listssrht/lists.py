from srht.app import get_projects
from srht.oauth import current_user
from listssrht.graphql import Visibility
from listssrht.types import ListAccess, Access

def get_access(ml, user=None):
    user = user or current_user

    # Anonymous
    if not user:
        if ml.visibility == Visibility.PRIVATE:
            return ListAccess.none
        return ml.default_access

    # Owner
    if user.id == ml.owner_id:
        return ListAccess.all

    # Admin
    if user.user_type == UserType.admin:
        return ListAccess.all

    # ACL entry?
    user_access = Access.query.filter_by(list=ml, user=user).first()
    if user_access:
        return user_access.permissions

    if ml.visibility == Visibility.PRIVATE:
        return ListAccess.none
    return ml.default_access

def project_nav(mailing_list):
    projects = get_projects(mailing_list.owner, mailing_list.rid)
    return {
        "projects": projects,
        "owner": mailing_list.owner,
    }
