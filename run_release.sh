#/bin/bash
docker run -ti --env-file <(cat config.mk | sed 's#export ##') webmention
