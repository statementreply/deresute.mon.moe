// FIXME
// do more on javascript side
google.charts.setOnLoadCallback(drawDistCompareChart);
google.charts.setOnLoadCallback(pageChange2);
function pageChange2() {
    $(window).on("pagechange", function() {
        drawDistCompareChart();
    });
    $(window).on("orientationchange", function() {
        drawDistCompareChart();
    });
};

function drawDistCompareChart() {
    var myUri = document.documentURI;
    //setAspectRatio();
    currentPage = $("body").pagecontainer("getActivePage");
    var options = {
        hAxis: {
            title: "rank",
            minValue: 0.92,
            gridlines: {count: 10},
            format: 'short',
            viewWindow: {max: 320000},
        },
        vAxis: {
            minValue: 0,
            textPosition: 'in',
            logScale: true,
            format: 'short',
            //gridlines: {count: 5},
            //minorGridlines: {count: 1},
            viewWindow: {min: 8000},
        },
        series: {
            0: {targetAxisIndex: 0},
        },
        vAxes: {
            0: {title: 'pt'},
        },
        interpolateNulls: true,
        explorer: {maxZoomIn: 0.1},
        chartArea: {width: '100%', height: '65%'},
        legend: {position: 'top', alignment: 'center'},
    };
    options.lineWidth = 0;
    options.pointSize = 2;

    console.log("myUri");
    console.log(myUri);
    if (myUri.indexOf("hlog") > 0) {
        console.log("set hlog");
        options.hAxis.logScale = true;
        //options.hAxis.scaleType = 'mirrorLog';
    }

    myDistChart = $("#myDistChart", currentPage);
    if (myDistChart.length == 0) {
        return;
    }
    setAspectRatio_i(myDistChart);
    eventName1 = "ラブレター";
    ddataurl1 = "/d_dist?t=1474945320";
    eventName2 = "命燃やして恋せよ乙女";
    ddataurl2 = "/d_dist?t=1484017320";
    eventName3 = "あんきら ! ？狂騒曲";
    ddataurl3 = "/d_dist?t=1482894120";
    eventName4 = "Lunatic Show";
    ddataurl4 = "/d_dist?t=1485572520";
    //eventName3 = ""
    var core = function (data1, data2, data3, data4) {
        var data_list = [];
        plot_label = [eventName1, eventName2, eventName3, eventName4];
        // t == 0
        for (t=0; t<1; t++) {
            dt = {"cols": [{"id":"rank","label":"rank","type":"number"}],
                "rows":[]}
            // cols
            dt["cols"].push({"id":plot_label[0], "label":plot_label[0], "type":"number"});
            dt["cols"].push({"id":plot_label[1], "label":plot_label[1], "type":"number"});
            dt["cols"].push({"id":plot_label[2], "label":plot_label[2], "type":"number"});
            dt["cols"].push({"id":plot_label[3], "label":plot_label[3], "type":"number"});
            cur1 = data1[t];
            cur2 = data2[t];
            cur3 = data3[t];
            cur4 = data4[t];
            //cur2 = data[t+2];
            var rankMap = new Map();

            // rows
            for (i=0; i<cur1.length; i++) {
                row = cur1[i];
                rankMap.set(row[0], [row[1]]);
                console.log("here01");
            }
            for (i=0; i<cur2.length; i++) {
                row = cur2[i];
                orig = rankMap.get(row[0]);
                if (orig != undefined) {
                } else {
                    orig = [null];
                }
                orig.push(row[1]);
                rankMap.set(row[0], orig);
            }
            for (i=0; i<cur3.length; i++) {
                row = cur3[i];
                orig = rankMap.get(row[0]);
                if (orig != undefined) {
                } else {
                    orig = [null, null];
                }
                orig.push(row[1]);
                rankMap.set(row[0], orig);
            }
            for (i=0; i<cur4.length; i++) {
                row = cur4[i];
                orig = rankMap.get(row[0]);
                if (orig != undefined) {
                } else {
                    orig = [null, null, null];
                }
                orig.push(row[1]);
                rankMap.set(row[0], orig);
            }
            console.log(JSON.stringify(rankMap));
            console.log(rankMap);

            for (var [rank, row] of rankMap)
            {
                row_map = {"c": [
                    {"v": rank},
                    {"v": row[0]},
                    {"v": row[1]},
                    {"v": row[2]},
                    {"v": row[3]}
                ]};
                dt["rows"].push(row_map);
            }
            // t=0: dt: ranklist
            // t=1: dt: speedlist
            data_list[t] = dt;
        }
        var data_pt = new google.visualization.DataTable(data_list[0]);
        chart = new google.visualization.LineChart(myDistChart.get(0));
        //console.log(data_list[0]);
        //console.log(JSON.stringify(data_list[0]));
        chart.draw(data_pt, options);
    };
    $.getJSON(ddataurl1, "", function (data1) {
            $.getJSON(ddataurl2, "", function (data2) {
                $.getJSON(ddataurl3, "", function (data3) {
                    $.getJSON(ddataurl4, "", function (data4) {
                        core(data1, data2, data3, data4);
                    });
                });
            });
    });

}

