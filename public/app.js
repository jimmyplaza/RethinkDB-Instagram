
$(document).ready(function() {

  //var mapTiles = "http://{s}.tile.osm.org/{z}/{x}/{y}.png";
  //var mapTiles = "http://tile.stamen.com/watercolor/{z}/{x}/{y}.png";
  //var mapAttrib = "&copy; <a href=\"http://osm.org/copyright\">OpenStreetMap</a> contributors";

  //var map = L.map('map').setView([0, 0], 2);
        //mapLink =
            //'<a href="http://openstreetmap.org">OpenStreetMap</a>';
        //L.tileLayer(
            ////'http://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
            //'http://{s}.tile.stamen.com/watercolor/{z}/{x}/{y}.jpg', {
            //attribution: 'Map data &copy; ' + mapLink,
            //maxZoom: 18,
            //}).addTo(map);

// =================================================================

  var osmLink = '<a href="http://openstreetmap.org">OpenStreetMap</a>',
    thunLink = '<a href="http://thunderforest.com/">Thunderforest</a>';

  var osmUrl = 'http://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png',
    osmAttrib = '&copy; ' + osmLink + ' Contributors',
    waterUrl = 'http://{s}.tile.stamen.com/watercolor/{z}/{x}/{y}.jpg',
    thunAttrib = '&copy; '+osmLink+' Contributors & '+thunLink;

  var osmMap = L.tileLayer(osmUrl, {attribution: osmAttrib, maxZoom: 18}),
       waterMap = L.tileLayer(waterUrl, {attribution: thunAttrib, maxZoom: 18});

  var map = L.map('map', {
        layers: [waterMap] // only add one!
       })
       .setView([0, 0], 2);

  var baseLayers = {
      "OpenStreetMap": osmMap,
      "WaterColor": waterMap
    };

  L.control.layers(baseLayers).addTo(map);

  var template = Handlebars.compile($("#cat-template").html());
  var markers = [];

  $("#map").hide();

  $("#toggle").click(function(e) {
    $("#toggle .button").removeClass("selected");
    $(e.target).addClass("selected");

    if (e.target.id == "grid-button") $("#map").hide();
    else $("#map").show();
  });

  function addCat(cat) {
      console.log("in addCat.")
      //console.log(cat)
    cat.date = moment.unix(cat.created_time).format("MMM DD, h:mm a");
    $("#cats").prepend(template(cat));

    if (cat.place) {
      var count = markers.unshift(L.marker(L.latLng(
          cat.place.Point.Lat,
          cat.place.Point.Lon)));

      map.addLayer(markers[0]);
      markers[0].bindPopup(
          "<img src=\"" + cat.images.thumbnail.url + "\">",
          {minWidth: 150, minHeight: 150});

      markers[0].openPopup();

      if (count > 100)
        map.removeLayer(markers.pop());
    }
  }

  //var socket = io.connect('http://localhost');
  var socket = io();

  socket.on("cat", addCat); //for real time add cat
  socket.on("recent", function(data) { // add recent 12 cats on the photo wall
    data.reverse().forEach(addCat);
  });
  socket.on("disconnect", function() {
      console.log("disconnect.")
  });

});
