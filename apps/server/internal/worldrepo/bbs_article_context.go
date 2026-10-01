package worldrepo

import "zutto-pccom/apps/server/internal/world"

// Read existing canonical history directly; deleted conversation PoCs no longer
// own a second post lookup implementation.
func findCanonicalPostByID(posts []world.Post, id int64) (world.Post,bool) {
    for _,post:=range posts {
        if post.ID==id {return post,true}
    }
    return world.Post{},false
}
