# Stage 1: Build the Node-RED image with the custom extension
FROM nodered/node-red:latest AS build

# Install the sparkplug extension
RUN npm install node-red-contrib-mqtt-sparkplug-plus

# Switch to the root user. This is necessary because otherwise nodered
# cannot write to the data folder that is mounted in from compose file
USER root
ENV UID 0
ENV GID 0

# Start Node-RED with the user's data
CMD ["npm", "start", "--", "--userDir", "/data"]
