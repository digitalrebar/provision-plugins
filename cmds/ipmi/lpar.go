package main

import (
	"bytes"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"time"

	"github.com/digitalrebar/logger"
	"github.com/digitalrebar/provision/v4/models"
	"golang.org/x/net/publicsuffix"
)

type lpar struct {
	client                  *http.Client
	url, username, password string
}

/* Curl example of login
curl -k -c cookies.txt -i -X PUT -H "Content-Type: application/vnd.ibm.powervm.web+xml; type=LogonRequest" -H "Accept: application/vnd.ibm.powervm.web+xml; type=LogonResponse" -H "X-Audit-Memento: hmc_test" -d @login.xml https://129.40.108.10:12443/rest/api/web/Logon

<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<LogonRequest xmlns="http://www.ibm.com/xmlns/systems/power/firmware/web/mc/2012_10/" schemaVersion="V1_0">
  <UserID>b1p052</UserID>
  <Password>pwb1p052</Password>
</LogonRequest>
*/
type LogonRequest struct {
	XMLName       xml.Name `xml:"LogonRequest"`
	Text          string   `xml:",chardata"`
	Xmlns         string   `xml:"xmlns,attr"`
	SchemaVersion string   `xml:"schemaVersion,attr"`
	UserID        string   `xml:"UserID"`
	Password      string   `xml:"Password"`
}

/*
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<LogonResponse xmlns="http://www.ibm.com/xmlns/systems/power/firmware/web/mc/2012_10/" xmlns:ns2="http://www.w3.org/XML/1998/namespace/k2" schemaVersion="V1_0">
    <Metadata>
        <Atom/>
    </Metadata>
    <X-API-Session kb="ROR" kxe="false">fxKI31-nJ5rRTi965AktfMfOSAqNm8msJCRNVXI5JJ3PYq7biWvl0k3EGu9055X5bDOexLfCOfe7wcbhd_Q5wz9QxQttNbBb68ap-M-QupBeeVP6QoQxNnr6wGeVdy-Vsy0YTY1Bvv3AE8UEi0Xbo1j2dTY83t9yiGz6Ob3428U5VghIvj4W6zK7my8wC1KuZmA6Z075KmkgDWJ7lCKF229_7xq-GN_J5tiE1zwSXvY=</X-API-Session>
</LogonResponse>
*/
type LogonResponse struct {
	XMLName       xml.Name `xml:"LogonResponse"`
	Text          string   `xml:",chardata"`
	Xmlns         string   `xml:"xmlns,attr"`
	SchemaVersion string   `xml:"schemaVersion,attr"`
	Metadata      struct {
		Text string `xml:",chardata"`
		Atom string `xml:"Atom"`
	} `xml:"Metadata"`
	XAPISession struct {
		Text string `xml:",chardata"`
		Kb   string `xml:"kb,attr"`
		Kxe  string `xml:"kxe,attr"`
	} `xml:"X-API-Session"`
}

/*
Curl example to PowerOn
curl -k -c cookies.txt -b cookies.txt -i -H "Accept: application/atom+xml; charset=UTF-8" https://129.40.108.10:12443/rest/api/uom/LogicalPartition/48EB2A9F-6028-41C6-8D2F-6A539361BB29/do/PowerOn -X PUT -d @power-on.xml  -H "Content-Type: application/vnd.ibm.powervm.web+xml; type=JobRequest"

Curl example to PowerOff
curl -k -c cookies.txt -b cookies.txt -i -H "Accept: application/atom+xml; charset=UTF-8" https://129.40.108.10:12443/rest/api/uom/LogicalPartition/48EB2A9F-6028-41C6-8D2F-6A539361BB29/do/PowerOff -X PUT -d @power-off.xml -H "Content-Type: application/vnd.ibm.powervm.web+xml; type=JobRequest"
*/

/* Job Request - PowerOn
<JobRequest
 xmlns="http://www.ibm.com/xmlns/systems/power/firmware/web/mc/2012_10/"
 xmlns:ns2="http://www.w3.org/XML/1998/namespace/k2" schemaVersion="V1_1_0">
    <Metadata>
        <Atom/>
    </Metadata>
    <RequestedOperation kb="CUR" kxe="false" schemaVersion="V1_0">
        <Metadata>
            <Atom/>
        </Metadata>
        <OperationName kb="ROR" kxe="false">PowerOn</OperationName>
        <GroupName kb="ROR" kxe="false">LogicalPartition</GroupName>
    </RequestedOperation>
    <JobParameters kb="CUR" kxe="false" schemaVersion="V1_0">
        <Metadata>
            <Atom/>
        </Metadata>
	  <JobParameter schemaVersion="V1_0">
           <Metadata>
               <Atom/>
           </Metadata>
           <ParameterName kxe="false" kb="ROR">novsi</ParameterName>
           <ParameterValue kxe="false" kb="CUR">true</ParameterValue>
      </JobParameter>
	  <JobParameter schemaVersion="V1_0">
           <Metadata>
               <Atom/>
           </Metadata>
           <ParameterName kxe="false" kb="ROR">force</ParameterName>
           <ParameterValue kxe="false" kb="CUR">false</ParameterValue>
      </JobParameter>
		<JobParameter schemaVersion="V1_0">
            <Metadata>
                <Atom/>
            </Metadata>
            <ParameterName kxe="false" kb="ROR">bootmode</ParameterName>
            <ParameterValue kxe="false" kb="CUR">norm</ParameterValue>
        </JobParameter>
   </JobParameters>
</JobRequest:JobRequest>
*/
/* JobRequest - Off
  <JobRequest
 xmlns="http://www.ibm.com/xmlns/systems/power/firmware/web/mc/2012_10/"
 xmlns:ns2="http://www.w3.org/XML/1998/namespace/k2" schemaVersion="V1_0">
	    <Metadata>
	        <Atom/>
	    </Metadata>
	    <RequestedOperation kxe="false" kb="CUR" schemaVersion="V1_0">
	        <Metadata>
	            <Atom/>
	        </Metadata>
	        <OperationName kxe="false" kb="ROR">PowerOff</OperationName>
	        <GroupName kxe="false" kb="ROR">LogicalPartition</GroupName>
	        <ProgressType kxe="false" kb="ROR">DISCRETE</ProgressType>
	    </RequestedOperation>
	    <JobParameters kxe="false" kb="CUR" schemaVersion="V1_0">
	        <Metadata>
	            <Atom/>
	        </Metadata>
	        <JobParameter schemaVersion="V1_0">
	            <Metadata>
	                <Atom/>
	            </Metadata>
	            <ParameterName kxe="false" kb="ROR">immediate</ParameterName>
	            <ParameterValue kxe="false" kb="CUR">true</ParameterValue>
	        </JobParameter>
	        <JobParameter schemaVersion="V1_0">
	            <Metadata>
	                <Atom/>
	            </Metadata>
	            <ParameterName kxe="false" kb="ROR">restart</ParameterName>
	            <ParameterValue kxe="false" kb="CUR">false</ParameterValue>
	        </JobParameter>
	        <JobParameter schemaVersion="V1_0">
	            <Metadata>
	                <Atom/>
	            </Metadata>
	            <ParameterName kxe="false" kb="ROR">operation</ParameterName>
	            <ParameterValue kxe="false" kb="CUR">shutdown</ParameterValue>
	        </JobParameter>
	    </JobParameters>
	</JobRequest>
*/
type JobRequest struct {
	XMLName       xml.Name `xml:"JobRequest"`
	Text          string   `xml:",chardata"`
	Xmlns         string   `xml:"xmlns,attr"`
	SchemaVersion string   `xml:"schemaVersion,attr"`
	Metadata      struct {
		Text string `xml:",chardata"`
		Atom string `xml:"Atom"`
	} `xml:"Metadata"`
	RequestedOperation struct {
		Text          string `xml:",chardata"`
		Kb            string `xml:"kb,attr"`
		Kxe           string `xml:"kxe,attr"`
		SchemaVersion string `xml:"schemaVersion,attr"`
		Metadata      struct {
			Text string `xml:",chardata"`
			Atom string `xml:"Atom"`
		} `xml:"Metadata"`
		OperationName struct {
			Text string `xml:",chardata"`
			Kb   string `xml:"kb,attr"`
			Kxe  string `xml:"kxe,attr"`
		} `xml:"OperationName"`
		GroupName struct {
			Text string `xml:",chardata"`
			Kb   string `xml:"kb,attr"`
			Kxe  string `xml:"kxe,attr"`
		} `xml:"GroupName"`
		ProgressType struct {
			Text string `xml:",chardata"`
			Kb   string `xml:"kb,attr"`
			Kxe  string `xml:"kxe,attr"`
		} `xml:"ProgressType"`
	} `xml:"RequestedOperation"`
	JobParameters struct {
		Text          string `xml:",chardata"`
		Kb            string `xml:"kb,attr"`
		Kxe           string `xml:"kxe,attr"`
		SchemaVersion string `xml:"schemaVersion,attr"`
		Metadata      struct {
			Text string `xml:",chardata"`
			Atom string `xml:"Atom"`
		} `xml:"Metadata"`
		JobParameter []JobParameter `xml:"JobParameter"`
	} `xml:"JobParameters"`
}

type JobParameter struct {
	Text          string `xml:",chardata"`
	SchemaVersion string `xml:"schemaVersion,attr"`
	Metadata      struct {
		Text string `xml:",chardata"`
		Atom string `xml:"Atom"`
	} `xml:"Metadata"`
	ParameterName struct {
		Text string `xml:",chardata"`
		Kxe  string `xml:"kxe,attr"`
		Kb   string `xml:"kb,attr"`
	} `xml:"ParameterName"`
	ParameterValue struct {
		Text string `xml:",chardata"`
		Kxe  string `xml:"kxe,attr"`
		Kb   string `xml:"kb,attr"`
	} `xml:"ParameterValue"`
}

/* JobResponse
<entry xmlns="http://www.w3.org/2005/Atom" xmlns:ns2="http://a9.com/-/spec/opensearch/1.1/" xmlns:ns3="http://www.w3.org/1999/xhtml">
    <id>b8c4e4f2-7203-4ecf-a6bb-2ade833bf0d2</id>
    <title>JobResponse</title>
    <published>2021-07-28T15:56:49.018Z</published>
    <link rel="SELF" href="https://129.40.108.10:12443/rest/api/uom/jobs/1622235308406"/>
    <author>
        <name>IBM Power Systems Management Console</name>
    </author>
    <content type="application/vnd.ibm.powervm.web+xml; type=JobResponse">
        <JobResponse:JobResponse xmlns:JobResponse="http://www.ibm.com/xmlns/systems/power/firmware/web/mc/2012_10/" xmlns="http://www.ibm.com/xmlns/systems/power/firmware/web/mc/2012_10/" xmlns:ns2="http://www.w3.org/XML/1998/namespace/k2" schemaVersion="V1_0">
    <Metadata>
        <Atom/>
    </Metadata>
    <RequestURL kb="ROR" kxe="false" href="LogicalPartition/48EB2A9F-6028-41C6-8D2F-6A539361BB29/do/PowerOn" rel="via" title="The URL to which the JobRequest was submitted."/>
    <TargetUuid kxe="false" kb="ROR">48EB2A9F-6028-41C6-8D2F-6A539361BB29</TargetUuid>
    <JobID kb="ROR" kxe="false">1622235308406</JobID>
    <TimeStarted kxe="false" kb="ROR">0</TimeStarted>
    <TimeCompleted kb="ROR" kxe="false">0</TimeCompleted>
    <Status kxe="false" kb="ROR">NOT_STARTED</Status>
    <JobRequestInstance kb="ROR" kxe="false" schemaVersion="V1_0">
        <Metadata>
            <Atom/>
        </Metadata>
        <RequestedOperation kxe="false" kb="CUR" schemaVersion="V1_0">
            <Metadata>
                <Atom/>
            </Metadata>
            <OperationName kxe="false" kb="ROR">PowerOn</OperationName>
            <GroupName kb="ROR" kxe="false">LogicalPartition</GroupName>
        </RequestedOperation>
        <JobParameters kxe="false" kb="CUR" schemaVersion="V1_0">
            <Metadata>
                <Atom/>
            </Metadata>
            <JobParameter schemaVersion="V1_0">
                <Metadata>
                    <Atom/>
                </Metadata>
                <ParameterName kb="ROR" kxe="false">novsi</ParameterName>
                <ParameterValue kb="CUR" kxe="false">true</ParameterValue>
            </JobParameter>
            <JobParameter schemaVersion="V1_0">
                <Metadata>
                    <Atom/>
                </Metadata>
                <ParameterName kb="ROR" kxe="false">force</ParameterName>
                <ParameterValue kb="CUR" kxe="false">false</ParameterValue>
            </JobParameter>
            <JobParameter schemaVersion="V1_0">
                <Metadata>
                    <Atom/>
                </Metadata>
                <ParameterName kb="ROR" kxe="false">bootmode</ParameterName>
                <ParameterValue kb="CUR" kxe="false">norm</ParameterValue>
            </JobParameter>
        </JobParameters>
    </JobRequestInstance>
    <Progress kxe="false" kb="ROO" schemaVersion="V1_0">
        <Metadata>
            <Atom/>
        </Metadata>
    </Progress>
    <Results kb="ROR" kxe="false" schemaVersion="V1_0">
        <Metadata>
            <Atom/>
        </Metadata>
    </Results>
</JobResponse:JobResponse>
    </content>
</entry>
*/

type JobEntry struct {
	XMLName   xml.Name `xml:"entry"`
	Text      string   `xml:",chardata"`
	Xmlns     string   `xml:"xmlns,attr"`
	Ns3       string   `xml:"ns3,attr"`
	ID        string   `xml:"id"`
	Title     string   `xml:"title"`
	Published string   `xml:"published"`
	Link      struct {
		Text string `xml:",chardata"`
		Rel  string `xml:"rel,attr"`
		Href string `xml:"href,attr"`
	} `xml:"link"`
	Author struct {
		Text string `xml:",chardata"`
		Name string `xml:"name"`
	} `xml:"author"`
	Content struct {
		Text        string `xml:",chardata"`
		Type        string `xml:"type,attr"`
		JobResponse struct {
			Text          string `xml:",chardata"`
			JobResponse   string `xml:"JobResponse,attr"`
			Xmlns         string `xml:"xmlns,attr"`
			SchemaVersion string `xml:"schemaVersion,attr"`
			Metadata      struct {
				Text string `xml:",chardata"`
				Atom string `xml:"Atom"`
			} `xml:"Metadata"`
			RequestURL struct {
				Text  string `xml:",chardata"`
				Kb    string `xml:"kb,attr"`
				Kxe   string `xml:"kxe,attr"`
				Href  string `xml:"href,attr"`
				Rel   string `xml:"rel,attr"`
				Title string `xml:"title,attr"`
			} `xml:"RequestURL"`
			TargetUuid struct {
				Text string `xml:",chardata"`
				Kxe  string `xml:"kxe,attr"`
				Kb   string `xml:"kb,attr"`
			} `xml:"TargetUuid"`
			JobID struct {
				Text string `xml:",chardata"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"JobID"`
			TimeStarted struct {
				Text string `xml:",chardata"`
				Kxe  string `xml:"kxe,attr"`
				Kb   string `xml:"kb,attr"`
			} `xml:"TimeStarted"`
			TimeCompleted struct {
				Text string `xml:",chardata"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"TimeCompleted"`
			Status struct {
				Text string `xml:",chardata"`
				Kxe  string `xml:"kxe,attr"`
				Kb   string `xml:"kb,attr"`
			} `xml:"Status"`
			JobRequestInstance struct {
				Text          string `xml:",chardata"`
				Kb            string `xml:"kb,attr"`
				Kxe           string `xml:"kxe,attr"`
				SchemaVersion string `xml:"schemaVersion,attr"`
				Metadata      struct {
					Text string `xml:",chardata"`
					Atom string `xml:"Atom"`
				} `xml:"Metadata"`
				RequestedOperation struct {
					Text          string `xml:",chardata"`
					Kxe           string `xml:"kxe,attr"`
					Kb            string `xml:"kb,attr"`
					SchemaVersion string `xml:"schemaVersion,attr"`
					Metadata      struct {
						Text string `xml:",chardata"`
						Atom string `xml:"Atom"`
					} `xml:"Metadata"`
					OperationName struct {
						Text string `xml:",chardata"`
						Kxe  string `xml:"kxe,attr"`
						Kb   string `xml:"kb,attr"`
					} `xml:"OperationName"`
					GroupName struct {
						Text string `xml:",chardata"`
						Kb   string `xml:"kb,attr"`
						Kxe  string `xml:"kxe,attr"`
					} `xml:"GroupName"`
				} `xml:"RequestedOperation"`
				JobParameters struct {
					Text          string `xml:",chardata"`
					Kxe           string `xml:"kxe,attr"`
					Kb            string `xml:"kb,attr"`
					SchemaVersion string `xml:"schemaVersion,attr"`
					Metadata      struct {
						Text string `xml:",chardata"`
						Atom string `xml:"Atom"`
					} `xml:"Metadata"`
					JobParameter []struct {
						Text          string `xml:",chardata"`
						SchemaVersion string `xml:"schemaVersion,attr"`
						Metadata      struct {
							Text string `xml:",chardata"`
							Atom string `xml:"Atom"`
						} `xml:"Metadata"`
						ParameterName struct {
							Text string `xml:",chardata"`
							Kb   string `xml:"kb,attr"`
							Kxe  string `xml:"kxe,attr"`
						} `xml:"ParameterName"`
						ParameterValue struct {
							Text string `xml:",chardata"`
							Kb   string `xml:"kb,attr"`
							Kxe  string `xml:"kxe,attr"`
						} `xml:"ParameterValue"`
					} `xml:"JobParameter"`
				} `xml:"JobParameters"`
			} `xml:"JobRequestInstance"`
			Progress struct {
				Text          string `xml:",chardata"`
				Kxe           string `xml:"kxe,attr"`
				Kb            string `xml:"kb,attr"`
				SchemaVersion string `xml:"schemaVersion,attr"`
				Metadata      struct {
					Text string `xml:",chardata"`
					Atom string `xml:"Atom"`
				} `xml:"Metadata"`
			} `xml:"Progress"`
			Results struct {
				Text          string `xml:",chardata"`
				Kb            string `xml:"kb,attr"`
				Kxe           string `xml:"kxe,attr"`
				SchemaVersion string `xml:"schemaVersion,attr"`
				Metadata      struct {
					Text string `xml:",chardata"`
					Atom string `xml:"Atom"`
				} `xml:"Metadata"`
			} `xml:"Results"`
		} `xml:"JobResponse"`
	} `xml:"content"`
}

/*
<entry xmlns="http://www.w3.org/2005/Atom" xmlns:ns2="http://a9.com/-/spec/opensearch/1.1/" xmlns:ns3="http://www.w3.org/1999/xhtml">
    <id>48EB2A9F-6028-41C6-8D2F-6A539361BB29</id>
    <title>LogicalPartition</title>
    <published>2021-07-29T02:30:24.937Z</published>
    <link rel="SELF" href="https://129.40.108.10:12443/rest/api/uom/LogicalPartition/48EB2A9F-6028-41C6-8D2F-6A539361BB29"/>
    <link rel="MANAGEMENT_CONSOLE" href="https://129.40.108.10:12443/rest/api/uom/ManagementConsole/37dfe310-cee2-3ffb-b30e-54c816105a38"/>
    <author>
        <name>IBM Power Systems Management Console</name>
    </author>
    <etag:etag xmlns:etag="http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/" xmlns="http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/">-1666866283</etag:etag>
    <content type="application/vnd.ibm.powervm.uom+xml; type=LogicalPartition">
        <LogicalPartition:LogicalPartition xmlns:LogicalPartition="http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/" xmlns="http://www.ibm.com/xmlns/systems/power/firmware/uom/mc/2012_10/" xmlns:ns2="http://www.w3.org/XML/1998/namespace/k2" schemaVersion="V1_0">
    <Metadata>
        <Atom>
            <AtomID>48EB2A9F-6028-41C6-8D2F-6A539361BB29</AtomID>
            <AtomCreated>1627525539116</AtomCreated>
        </Atom>
    </Metadata>
    <AllowPerformanceDataCollection kb="CUD" kxe="false">false</AllowPerformanceDataCollection>
    <AssociatedPartitionProfile kxe="false" kb="CUD" href="https://129.40.108.10:12443/rest/api/uom/LogicalPartition/48EB2A9F-6028-41C6-8D2F-6A539361BB29/LogicalPartitionProfile/a4ec7450-cc09-3502-aa93-1ad8d630ec80" rel="related"/>
    <AvailabilityPriority kb="UOD" kxe="false">127</AvailabilityPriority>
    <CurrentProcessorCompatibilityMode kxe="false" kb="ROO">POWER8</CurrentProcessorCompatibilityMode>
    <CurrentProfileSync kb="CUD" kxe="false">On</CurrentProfileSync>
    <IsBootable ksv="V1_3_0" kxe="false" kb="ROO">true</IsBootable>
    <IsConnectionMonitoringEnabled kb="UOD" kxe="false">true</IsConnectionMonitoringEnabled>
    <IsOperationInProgress kb="ROR" kxe="false">false</IsOperationInProgress>
    <IsRedundantErrorPathReportingEnabled kxe="false" kb="UOD">false</IsRedundantErrorPathReportingEnabled>
    <IsTimeReferencePartition kb="UOD" kxe="false">false</IsTimeReferencePartition>
    <IsVirtualServiceAttentionLEDOn kb="ROR" kxe="false">false</IsVirtualServiceAttentionLEDOn>
    <IsVirtualTrustedPlatformModuleEnabled kxe="false" kb="UOD">false</IsVirtualTrustedPlatformModuleEnabled>
    <KeylockPosition kb="CUD" kxe="false">normal</KeylockPosition>
    <LogicalSerialNumber kb="ROR" kxe="false">212169A3</LogicalSerialNumber>
    <OperatingSystemVersion kxe="false" kb="ROR">Unknown</OperatingSystemVersion>
    <PartitionCapabilities kb="ROR" kxe="false" schemaVersion="V1_0">
        <Metadata>
            <Atom/>
        </Metadata>
        <DynamicLogicalPartitionIOCapable kxe="false" kb="ROR">false</DynamicLogicalPartitionIOCapable>
        <DynamicLogicalPartitionMemoryCapable kxe="false" kb="ROR">false</DynamicLogicalPartitionMemoryCapable>
        <DynamicLogicalPartitionProcessorCapable kxe="false" kb="ROR">false</DynamicLogicalPartitionProcessorCapable>
        <InternalAndExternalIntrusionDetectionCapable kb="CUD" kxe="false">false</InternalAndExternalIntrusionDetectionCapable>
        <ResourceMonitoringControlOperatingSystemShutdownCapable kb="CUD" kxe="false">false</ResourceMonitoringControlOperatingSystemShutdownCapable>
    </PartitionCapabilities>
    <PartitionID kxe="false" kb="COD">3</PartitionID>
    <PartitionIOConfiguration kb="CUD" kxe="false" schemaVersion="V1_0">
        <Metadata>
            <Atom/>
        </Metadata>
        <MaximumVirtualIOSlots kxe="false" kb="CUD">64</MaximumVirtualIOSlots>
        <ProfileIOSlots kb="CUD" kxe="false" schemaVersion="V1_0">
            <Metadata>
                <Atom/>
            </Metadata>
        </ProfileIOSlots>
        <CurrentMaximumVirtualIOSlots kb="ROR" kxe="false">64</CurrentMaximumVirtualIOSlots>
    </PartitionIOConfiguration>
    <PartitionMemoryConfiguration kb="CUD" kxe="false" schemaVersion="V1_0">
        <Metadata>
            <Atom/>
        </Metadata>
        <ActiveMemoryExpansionEnabled kxe="false" kb="CUD">false</ActiveMemoryExpansionEnabled>
        <ActiveMemorySharingEnabled kb="CUD" kxe="false">false</ActiveMemorySharingEnabled>
        <DesiredHugePageCount kb="CUD" kxe="false">0</DesiredHugePageCount>
        <DesiredMemory kb="CUD" kxe="false">32768</DesiredMemory>
        <ExpansionFactor kxe="false" kb="CUD">0.0</ExpansionFactor>
        <HardwarePageTableRatio kxe="false" kb="CUD">7</HardwarePageTableRatio>
        <MaximumHugePageCount kb="CUD" kxe="false">0</MaximumHugePageCount>
        <MaximumMemory kxe="false" kb="CUD">34816</MaximumMemory>
        <MinimumHugePageCount kxe="false" kb="CUD">0</MinimumHugePageCount>
        <MinimumMemory kxe="false" kb="CUD">16384</MinimumMemory>
        <CurrentExpansionFactor kxe="false" kb="ROR">0.0</CurrentExpansionFactor>
        <CurrentHardwarePageTableRatio kxe="false" kb="ROR">7</CurrentHardwarePageTableRatio>
        <CurrentHugePageCount kxe="false" kb="ROR">0</CurrentHugePageCount>
        <CurrentMaximumHugePageCount kb="ROR" kxe="false">0</CurrentMaximumHugePageCount>
        <CurrentMaximumMemory kb="ROR" kxe="false">34816</CurrentMaximumMemory>
        <CurrentMemory kxe="false" kb="ROR">32768</CurrentMemory>
        <CurrentMinimumHugePageCount kb="ROR" kxe="false">0</CurrentMinimumHugePageCount>
        <CurrentMinimumMemory kxe="false" kb="ROR">16384</CurrentMinimumMemory>
        <MemoryExpansionHardwareAccessEnabled kxe="false" kb="ROR">true</MemoryExpansionHardwareAccessEnabled>
        <MemoryEncryptionHardwareAccessEnabled kb="ROR" kxe="false">true</MemoryEncryptionHardwareAccessEnabled>
        <MemoryExpansionEnabled kxe="false" kb="ROR">false</MemoryExpansionEnabled>
        <RedundantErrorPathReportingEnabled kb="ROR" kxe="false">false</RedundantErrorPathReportingEnabled>
        <RuntimeHugePageCount kxe="false" kb="ROR">0</RuntimeHugePageCount>
        <RuntimeMemory kxe="false" kb="ROR">32768</RuntimeMemory>
        <RuntimeMinimumMemory kb="ROR" kxe="false">16384</RuntimeMinimumMemory>
        <SharedMemoryEnabled kb="CUD" kxe="false">false</SharedMemoryEnabled>
        <PhysicalPageTableRatio ksv="V1_6_0" kxe="false" kb="CUD">6</PhysicalPageTableRatio>
    </PartitionMemoryConfiguration>
    <PartitionName kb="CUR" kxe="false">b1p052_Target-8c0bd1b0-0000084e</PartitionName>
    <PartitionProcessorConfiguration kb="CUD" kxe="false" schemaVersion="V1_0">
        <Metadata>
            <Atom/>
        </Metadata>
        <HasDedicatedProcessors kxe="false" kb="CUD">false</HasDedicatedProcessors>
        <SharedProcessorConfiguration kb="CUD" kxe="false" schemaVersion="V1_0">
            <Metadata>
                <Atom/>
            </Metadata>
            <DesiredProcessingUnits kb="CUD" kxe="false">0.5</DesiredProcessingUnits>
            <DesiredVirtualProcessors kb="CUD" kxe="false">4</DesiredVirtualProcessors>
            <MaximumProcessingUnits kxe="false" kb="CUD">8</MaximumProcessingUnits>
            <MaximumVirtualProcessors kb="CUD" kxe="false">8</MaximumVirtualProcessors>
            <MinimumProcessingUnits kxe="false" kb="CUD">0.1</MinimumProcessingUnits>
            <MinimumVirtualProcessors kb="CUD" kxe="false">1</MinimumVirtualProcessors>
            <SharedProcessorPoolID kb="CUD" kxe="false">0</SharedProcessorPoolID>
            <UncappedWeight kxe="false" kb="CUD">128</UncappedWeight>
        </SharedProcessorConfiguration>
        <SharingMode kb="CUD" kxe="false">uncapped</SharingMode>
        <CurrentHasDedicatedProcessors kxe="false" kb="ROR">false</CurrentHasDedicatedProcessors>
        <CurrentSharingMode kxe="false" kb="ROR">uncapped</CurrentSharingMode>
        <RuntimeHasDedicatedProcessors kxe="false" kb="ROR">false</RuntimeHasDedicatedProcessors>
        <CurrentSharedProcessorConfiguration kb="ROR" kxe="false" schemaVersion="V1_0">
            <Metadata>
                <Atom/>
            </Metadata>
            <AllocatedVirtualProcessors kb="ROR" kxe="false">4</AllocatedVirtualProcessors>
            <CurrentMaximumProcessingUnits kxe="false" kb="ROR">8</CurrentMaximumProcessingUnits>
            <CurrentMinimumProcessingUnits kb="ROR" kxe="false">0.1</CurrentMinimumProcessingUnits>
            <CurrentProcessingUnits kb="ROR" kxe="false">0.5</CurrentProcessingUnits>
            <CurrentSharedProcessorPoolID kxe="false" kb="ROR">0</CurrentSharedProcessorPoolID>
            <CurrentUncappedWeight kxe="false" kb="ROR">128</CurrentUncappedWeight>
            <CurrentMinimumVirtualProcessors kxe="false" kb="ROR">1</CurrentMinimumVirtualProcessors>
            <CurrentMaximumVirtualProcessors kb="ROR" kxe="false">8</CurrentMaximumVirtualProcessors>
            <RuntimeProcessingUnits kxe="false" kb="ROR">0.5</RuntimeProcessingUnits>
            <RuntimeUncappedWeight kxe="false" kb="ROR">128</RuntimeUncappedWeight>
        </CurrentSharedProcessorConfiguration>
    </PartitionProcessorConfiguration>
    <PartitionProfiles kb="CUD" kxe="false">
        <link href="https://129.40.108.10:12443/rest/api/uom/LogicalPartition/48EB2A9F-6028-41C6-8D2F-6A539361BB29/LogicalPartitionProfile/a4ec7450-cc09-3502-aa93-1ad8d630ec80" rel="related"/>
    </PartitionProfiles>
    <PartitionState kxe="false" kb="ROO">running</PartitionState>
    <PartitionType kb="COD" kxe="false">AIX/Linux</PartitionType>
    <PartitionUUID kb="ROO" kxe="false">48EB2A9F-6028-41C6-8D2F-6A539361BB29</PartitionUUID>
    <PendingProcessorCompatibilityMode kb="CUD" kxe="false">default</PendingProcessorCompatibilityMode>
    <ProgressPartitionDataRemaining kb="ROR" kxe="false">0</ProgressPartitionDataRemaining>
    <ProgressPartitionDataTotal kxe="false" kb="ROR">0</ProgressPartitionDataTotal>
    <ProgressState kb="ROR" kxe="false"/>
    <ResourceMonitoringControlState kb="ROR" kxe="false">inactive</ResourceMonitoringControlState>
    <AssociatedManagedSystem kb="CUD" kxe="false" href="https://129.40.108.10:12443/rest/api/uom/ManagedSystem/9a88901c-55cc-34ce-8f58-15e31ee53936" rel="related"/>
    <ClientNetworkAdapters kb="CUR" kxe="false">
        <link href="https://129.40.108.10:12443/rest/api/uom/LogicalPartition/48EB2A9F-6028-41C6-8D2F-6A539361BB29/ClientNetworkAdapter/12ad66cc-897e-33ea-a9c6-13c556cc6414" rel="related"/>
    </ClientNetworkAdapters>
    <HostEthernetAdapterLogicalPorts kxe="false" kb="CUD" schemaVersion="V1_0">
        <Metadata>
            <Atom/>
        </Metadata>
    </HostEthernetAdapterLogicalPorts>
    <MACAddressPrefix kb="ROR" kxe="false">46828295398912</MACAddressPrefix>
    <IsServicePartition kb="CUD" kxe="false">false</IsServicePartition>
    <PowerVMManagementCapable ksv="V1_5_0" kxe="false" kb="ROR">false</PowerVMManagementCapable>
    <ReferenceCode kxe="true" kb="ROO">Linux ppc64le</ReferenceCode>
    <AssignAllResources ksv="V1_6_0" kxe="false" kb="COD">false</AssignAllResources>
    <HardwareAcceleratorQoS ksv="V1_7_0" kxe="false" kb="CUD" schemaVersion="V1_0">
        <Metadata>
            <Atom/>
        </Metadata>
    </HardwareAcceleratorQoS>
    <LastActivatedProfile ksv="V1_7_0" kxe="false" kb="ROO">default_profile</LastActivatedProfile>
    <HasPhysicalIO ksv="V1_7_0" kb="ROO" kxe="false">false</HasPhysicalIO>
    <OperatingSystemType ksv="V1_8_0" kb="ROR" kxe="false">AIX/Linux</OperatingSystemType>
    <BootMode ksv="V1_10_1" kb="COD" kxe="false">Normal</BootMode>
    <PowerOnWithHypervisor ksv="V1_2_0" kxe="false" kb="CUD">false</PowerOnWithHypervisor>
    <MigrationStorageViosDataStatus ksv="V1_3_0" kxe="false" kb="ROR">Valid</MigrationStorageViosDataStatus>
    <MigrationStorageViosDataTimestamp ksv="V1_3_0" kxe="false" kb="ROR">Thu Jul 29 02:15:38 UTC 2021</MigrationStorageViosDataTimestamp>
    <RemoteRestartCapable kxe="false" kb="CUA">false</RemoteRestartCapable>
    <SimplifiedRemoteRestartCapable ksv="V1_2_0" kb="CUD" kxe="false">false</SimplifiedRemoteRestartCapable>
    <HasDedicatedProcessorsForMigration kxe="false" kb="ROR">false</HasDedicatedProcessorsForMigration>
    <SuspendCapable kb="CUD" kxe="false">false</SuspendCapable>
    <MigrationDisable ksv="V1_3_0" kb="CUD" kxe="false">false</MigrationDisable>
    <MigrationState kb="ROR" kxe="false">Not_Migrating</MigrationState>
    <RemoteRestartState kb="ROR" kxe="false">Invalid</RemoteRestartState>
    <VirtualFibreChannelClientAdapters kb="CUR" kxe="false">
        <link href="https://129.40.108.10:12443/rest/api/uom/LogicalPartition/48EB2A9F-6028-41C6-8D2F-6A539361BB29/VirtualFibreChannelClientAdapter/d488df22-4498-3cff-8451-4d763a0e0100" rel="related"/>
        <link href="https://129.40.108.10:12443/rest/api/uom/LogicalPartition/48EB2A9F-6028-41C6-8D2F-6A539361BB29/VirtualFibreChannelClientAdapter/a0cc19f3-bcf3-31b5-8a9f-43604694fe13" rel="related"/>
        <link href="https://129.40.108.10:12443/rest/api/uom/LogicalPartition/48EB2A9F-6028-41C6-8D2F-6A539361BB29/VirtualFibreChannelClientAdapter/4e5ffbf4-2285-3c4d-9e07-db2766c44027" rel="related"/>
        <link href="https://129.40.108.10:12443/rest/api/uom/LogicalPartition/48EB2A9F-6028-41C6-8D2F-6A539361BB29/VirtualFibreChannelClientAdapter/56cfa9d1-034f-34c0-aba2-1aa6c53edc0f" rel="related"/>
    </VirtualFibreChannelClientAdapters>
    <VirtualSCSIClientAdapters kxe="false" kb="CUR">
        <link href="https://129.40.108.10:12443/rest/api/uom/LogicalPartition/48EB2A9F-6028-41C6-8D2F-6A539361BB29/VirtualSCSIClientAdapter/d89e3ae4-cab4-3a9b-a2d5-3eea007f5b06" rel="related"/>
    </VirtualSCSIClientAdapters>
    <BootListInformation ksv="V1_5_0" kxe="false" kb="UOD" schemaVersion="V1_0">
        <Metadata>
            <Atom/>
        </Metadata>
        <PendingBootString group="Advanced" ksv="V1_5_0" kxe="false" kb="UOO"/>
        <BootDeviceList group="Advanced" ksv="V1_5_0" kb="ROO" kxe="false">/vdevice/v-scsi@30000002/disk@8100000000000000 /vdevice/vfc-client@30000003/disk@500507680c223077,0000000000000000 /vdevice/l-lan@30000020:speed=auto,duplex=auto,000.000.000.000,,000.000.000.000,000.000.000.000,5,5,000.000.000.000,512</BootDeviceList>
        <ShadowBootDeviceList group="Advanced" ksv="V1_5_0" kb="ROO" kxe="false">,IBM-2145-600507680c808185d000000000000b8c,</ShadowBootDeviceList>
        <LastBootedDeviceString group="Advanced" ksv="V1_5_0" kb="ROO" kxe="false">/vdevice/l-lan@30000020:speed=auto,duplex=auto,000.000.000.000,,000.000.000.000,000.000.000.000,5,5,000.000.000.000,512</LastBootedDeviceString>
    </BootListInformation>
</LogicalPartition:LogicalPartition>
    </content>
</entry>
*/

type GetEntry struct {
	XMLName   xml.Name `xml:"entry"`
	Text      string   `xml:",chardata"`
	Xmlns     string   `xml:"xmlns,attr"`
	Ns2       string   `xml:"ns2,attr"`
	Ns3       string   `xml:"ns3,attr"`
	ID        string   `xml:"id"`
	Title     string   `xml:"title"`
	Published string   `xml:"published"`
	Link      []struct {
		Text string `xml:",chardata"`
		Rel  string `xml:"rel,attr"`
		Href string `xml:"href,attr"`
	} `xml:"link"`
	Author struct {
		Text string `xml:",chardata"`
		Name string `xml:"name"`
	} `xml:"author"`
	Etag struct {
		Text  string `xml:",chardata"`
		Etag  string `xml:"etag,attr"`
		Xmlns string `xml:"xmlns,attr"`
	} `xml:"etag"`
	Content struct {
		Text             string `xml:",chardata"`
		Type             string `xml:"type,attr"`
		LogicalPartition struct {
			Text             string `xml:",chardata"`
			LogicalPartition string `xml:"LogicalPartition,attr"`
			Xmlns            string `xml:"xmlns,attr"`
			Ns2              string `xml:"ns2,attr"`
			SchemaVersion    string `xml:"schemaVersion,attr"`
			Metadata         struct {
				Text string `xml:",chardata"`
				Atom struct {
					Text        string `xml:",chardata"`
					AtomID      string `xml:"AtomID"`
					AtomCreated string `xml:"AtomCreated"`
				} `xml:"Atom"`
			} `xml:"Metadata"`
			AllowPerformanceDataCollection struct {
				Text string `xml:",chardata"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"AllowPerformanceDataCollection"`
			AssociatedPartitionProfile struct {
				Text string `xml:",chardata"`
				Kxe  string `xml:"kxe,attr"`
				Kb   string `xml:"kb,attr"`
				Href string `xml:"href,attr"`
				Rel  string `xml:"rel,attr"`
			} `xml:"AssociatedPartitionProfile"`
			AvailabilityPriority struct {
				Text string `xml:",chardata"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"AvailabilityPriority"`
			CurrentProcessorCompatibilityMode struct {
				Text string `xml:",chardata"`
				Kxe  string `xml:"kxe,attr"`
				Kb   string `xml:"kb,attr"`
			} `xml:"CurrentProcessorCompatibilityMode"`
			CurrentProfileSync struct {
				Text string `xml:",chardata"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"CurrentProfileSync"`
			IsBootable struct {
				Text string `xml:",chardata"`
				Ksv  string `xml:"ksv,attr"`
				Kxe  string `xml:"kxe,attr"`
				Kb   string `xml:"kb,attr"`
			} `xml:"IsBootable"`
			IsConnectionMonitoringEnabled struct {
				Text string `xml:",chardata"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"IsConnectionMonitoringEnabled"`
			IsOperationInProgress struct {
				Text string `xml:",chardata"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"IsOperationInProgress"`
			IsRedundantErrorPathReportingEnabled struct {
				Text string `xml:",chardata"`
				Kxe  string `xml:"kxe,attr"`
				Kb   string `xml:"kb,attr"`
			} `xml:"IsRedundantErrorPathReportingEnabled"`
			IsTimeReferencePartition struct {
				Text string `xml:",chardata"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"IsTimeReferencePartition"`
			IsVirtualServiceAttentionLEDOn struct {
				Text string `xml:",chardata"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"IsVirtualServiceAttentionLEDOn"`
			IsVirtualTrustedPlatformModuleEnabled struct {
				Text string `xml:",chardata"`
				Kxe  string `xml:"kxe,attr"`
				Kb   string `xml:"kb,attr"`
			} `xml:"IsVirtualTrustedPlatformModuleEnabled"`
			KeylockPosition struct {
				Text string `xml:",chardata"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"KeylockPosition"`
			LogicalSerialNumber struct {
				Text string `xml:",chardata"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"LogicalSerialNumber"`
			OperatingSystemVersion struct {
				Text string `xml:",chardata"`
				Kxe  string `xml:"kxe,attr"`
				Kb   string `xml:"kb,attr"`
			} `xml:"OperatingSystemVersion"`
			PartitionCapabilities struct {
				Text          string `xml:",chardata"`
				Kb            string `xml:"kb,attr"`
				Kxe           string `xml:"kxe,attr"`
				SchemaVersion string `xml:"schemaVersion,attr"`
				Metadata      struct {
					Text string `xml:",chardata"`
					Atom string `xml:"Atom"`
				} `xml:"Metadata"`
				DynamicLogicalPartitionIOCapable struct {
					Text string `xml:",chardata"`
					Kxe  string `xml:"kxe,attr"`
					Kb   string `xml:"kb,attr"`
				} `xml:"DynamicLogicalPartitionIOCapable"`
				DynamicLogicalPartitionMemoryCapable struct {
					Text string `xml:",chardata"`
					Kxe  string `xml:"kxe,attr"`
					Kb   string `xml:"kb,attr"`
				} `xml:"DynamicLogicalPartitionMemoryCapable"`
				DynamicLogicalPartitionProcessorCapable struct {
					Text string `xml:",chardata"`
					Kxe  string `xml:"kxe,attr"`
					Kb   string `xml:"kb,attr"`
				} `xml:"DynamicLogicalPartitionProcessorCapable"`
				InternalAndExternalIntrusionDetectionCapable struct {
					Text string `xml:",chardata"`
					Kb   string `xml:"kb,attr"`
					Kxe  string `xml:"kxe,attr"`
				} `xml:"InternalAndExternalIntrusionDetectionCapable"`
				ResourceMonitoringControlOperatingSystemShutdownCapable struct {
					Text string `xml:",chardata"`
					Kb   string `xml:"kb,attr"`
					Kxe  string `xml:"kxe,attr"`
				} `xml:"ResourceMonitoringControlOperatingSystemShutdownCapable"`
			} `xml:"PartitionCapabilities"`
			PartitionID struct {
				Text string `xml:",chardata"`
				Kxe  string `xml:"kxe,attr"`
				Kb   string `xml:"kb,attr"`
			} `xml:"PartitionID"`
			PartitionIOConfiguration struct {
				Text          string `xml:",chardata"`
				Kb            string `xml:"kb,attr"`
				Kxe           string `xml:"kxe,attr"`
				SchemaVersion string `xml:"schemaVersion,attr"`
				Metadata      struct {
					Text string `xml:",chardata"`
					Atom string `xml:"Atom"`
				} `xml:"Metadata"`
				MaximumVirtualIOSlots struct {
					Text string `xml:",chardata"`
					Kxe  string `xml:"kxe,attr"`
					Kb   string `xml:"kb,attr"`
				} `xml:"MaximumVirtualIOSlots"`
				ProfileIOSlots struct {
					Text          string `xml:",chardata"`
					Kb            string `xml:"kb,attr"`
					Kxe           string `xml:"kxe,attr"`
					SchemaVersion string `xml:"schemaVersion,attr"`
					Metadata      struct {
						Text string `xml:",chardata"`
						Atom string `xml:"Atom"`
					} `xml:"Metadata"`
				} `xml:"ProfileIOSlots"`
				CurrentMaximumVirtualIOSlots struct {
					Text string `xml:",chardata"`
					Kb   string `xml:"kb,attr"`
					Kxe  string `xml:"kxe,attr"`
				} `xml:"CurrentMaximumVirtualIOSlots"`
			} `xml:"PartitionIOConfiguration"`
			PartitionMemoryConfiguration struct {
				Text          string `xml:",chardata"`
				Kb            string `xml:"kb,attr"`
				Kxe           string `xml:"kxe,attr"`
				SchemaVersion string `xml:"schemaVersion,attr"`
				Metadata      struct {
					Text string `xml:",chardata"`
					Atom string `xml:"Atom"`
				} `xml:"Metadata"`
				ActiveMemoryExpansionEnabled struct {
					Text string `xml:",chardata"`
					Kxe  string `xml:"kxe,attr"`
					Kb   string `xml:"kb,attr"`
				} `xml:"ActiveMemoryExpansionEnabled"`
				ActiveMemorySharingEnabled struct {
					Text string `xml:",chardata"`
					Kb   string `xml:"kb,attr"`
					Kxe  string `xml:"kxe,attr"`
				} `xml:"ActiveMemorySharingEnabled"`
				DesiredHugePageCount struct {
					Text string `xml:",chardata"`
					Kb   string `xml:"kb,attr"`
					Kxe  string `xml:"kxe,attr"`
				} `xml:"DesiredHugePageCount"`
				DesiredMemory struct {
					Text string `xml:",chardata"`
					Kb   string `xml:"kb,attr"`
					Kxe  string `xml:"kxe,attr"`
				} `xml:"DesiredMemory"`
				ExpansionFactor struct {
					Text string `xml:",chardata"`
					Kxe  string `xml:"kxe,attr"`
					Kb   string `xml:"kb,attr"`
				} `xml:"ExpansionFactor"`
				HardwarePageTableRatio struct {
					Text string `xml:",chardata"`
					Kxe  string `xml:"kxe,attr"`
					Kb   string `xml:"kb,attr"`
				} `xml:"HardwarePageTableRatio"`
				MaximumHugePageCount struct {
					Text string `xml:",chardata"`
					Kb   string `xml:"kb,attr"`
					Kxe  string `xml:"kxe,attr"`
				} `xml:"MaximumHugePageCount"`
				MaximumMemory struct {
					Text string `xml:",chardata"`
					Kxe  string `xml:"kxe,attr"`
					Kb   string `xml:"kb,attr"`
				} `xml:"MaximumMemory"`
				MinimumHugePageCount struct {
					Text string `xml:",chardata"`
					Kxe  string `xml:"kxe,attr"`
					Kb   string `xml:"kb,attr"`
				} `xml:"MinimumHugePageCount"`
				MinimumMemory struct {
					Text string `xml:",chardata"`
					Kxe  string `xml:"kxe,attr"`
					Kb   string `xml:"kb,attr"`
				} `xml:"MinimumMemory"`
				CurrentExpansionFactor struct {
					Text string `xml:",chardata"`
					Kxe  string `xml:"kxe,attr"`
					Kb   string `xml:"kb,attr"`
				} `xml:"CurrentExpansionFactor"`
				CurrentHardwarePageTableRatio struct {
					Text string `xml:",chardata"`
					Kxe  string `xml:"kxe,attr"`
					Kb   string `xml:"kb,attr"`
				} `xml:"CurrentHardwarePageTableRatio"`
				CurrentHugePageCount struct {
					Text string `xml:",chardata"`
					Kxe  string `xml:"kxe,attr"`
					Kb   string `xml:"kb,attr"`
				} `xml:"CurrentHugePageCount"`
				CurrentMaximumHugePageCount struct {
					Text string `xml:",chardata"`
					Kb   string `xml:"kb,attr"`
					Kxe  string `xml:"kxe,attr"`
				} `xml:"CurrentMaximumHugePageCount"`
				CurrentMaximumMemory struct {
					Text string `xml:",chardata"`
					Kb   string `xml:"kb,attr"`
					Kxe  string `xml:"kxe,attr"`
				} `xml:"CurrentMaximumMemory"`
				CurrentMemory struct {
					Text string `xml:",chardata"`
					Kxe  string `xml:"kxe,attr"`
					Kb   string `xml:"kb,attr"`
				} `xml:"CurrentMemory"`
				CurrentMinimumHugePageCount struct {
					Text string `xml:",chardata"`
					Kb   string `xml:"kb,attr"`
					Kxe  string `xml:"kxe,attr"`
				} `xml:"CurrentMinimumHugePageCount"`
				CurrentMinimumMemory struct {
					Text string `xml:",chardata"`
					Kxe  string `xml:"kxe,attr"`
					Kb   string `xml:"kb,attr"`
				} `xml:"CurrentMinimumMemory"`
				MemoryExpansionHardwareAccessEnabled struct {
					Text string `xml:",chardata"`
					Kxe  string `xml:"kxe,attr"`
					Kb   string `xml:"kb,attr"`
				} `xml:"MemoryExpansionHardwareAccessEnabled"`
				MemoryEncryptionHardwareAccessEnabled struct {
					Text string `xml:",chardata"`
					Kb   string `xml:"kb,attr"`
					Kxe  string `xml:"kxe,attr"`
				} `xml:"MemoryEncryptionHardwareAccessEnabled"`
				MemoryExpansionEnabled struct {
					Text string `xml:",chardata"`
					Kxe  string `xml:"kxe,attr"`
					Kb   string `xml:"kb,attr"`
				} `xml:"MemoryExpansionEnabled"`
				RedundantErrorPathReportingEnabled struct {
					Text string `xml:",chardata"`
					Kb   string `xml:"kb,attr"`
					Kxe  string `xml:"kxe,attr"`
				} `xml:"RedundantErrorPathReportingEnabled"`
				RuntimeHugePageCount struct {
					Text string `xml:",chardata"`
					Kxe  string `xml:"kxe,attr"`
					Kb   string `xml:"kb,attr"`
				} `xml:"RuntimeHugePageCount"`
				RuntimeMemory struct {
					Text string `xml:",chardata"`
					Kxe  string `xml:"kxe,attr"`
					Kb   string `xml:"kb,attr"`
				} `xml:"RuntimeMemory"`
				RuntimeMinimumMemory struct {
					Text string `xml:",chardata"`
					Kb   string `xml:"kb,attr"`
					Kxe  string `xml:"kxe,attr"`
				} `xml:"RuntimeMinimumMemory"`
				SharedMemoryEnabled struct {
					Text string `xml:",chardata"`
					Kb   string `xml:"kb,attr"`
					Kxe  string `xml:"kxe,attr"`
				} `xml:"SharedMemoryEnabled"`
				PhysicalPageTableRatio struct {
					Text string `xml:",chardata"`
					Ksv  string `xml:"ksv,attr"`
					Kxe  string `xml:"kxe,attr"`
					Kb   string `xml:"kb,attr"`
				} `xml:"PhysicalPageTableRatio"`
			} `xml:"PartitionMemoryConfiguration"`
			PartitionName struct {
				Text string `xml:",chardata"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"PartitionName"`
			PartitionProcessorConfiguration struct {
				Text          string `xml:",chardata"`
				Kb            string `xml:"kb,attr"`
				Kxe           string `xml:"kxe,attr"`
				SchemaVersion string `xml:"schemaVersion,attr"`
				Metadata      struct {
					Text string `xml:",chardata"`
					Atom string `xml:"Atom"`
				} `xml:"Metadata"`
				HasDedicatedProcessors struct {
					Text string `xml:",chardata"`
					Kxe  string `xml:"kxe,attr"`
					Kb   string `xml:"kb,attr"`
				} `xml:"HasDedicatedProcessors"`
				SharedProcessorConfiguration struct {
					Text          string `xml:",chardata"`
					Kb            string `xml:"kb,attr"`
					Kxe           string `xml:"kxe,attr"`
					SchemaVersion string `xml:"schemaVersion,attr"`
					Metadata      struct {
						Text string `xml:",chardata"`
						Atom string `xml:"Atom"`
					} `xml:"Metadata"`
					DesiredProcessingUnits struct {
						Text string `xml:",chardata"`
						Kb   string `xml:"kb,attr"`
						Kxe  string `xml:"kxe,attr"`
					} `xml:"DesiredProcessingUnits"`
					DesiredVirtualProcessors struct {
						Text string `xml:",chardata"`
						Kb   string `xml:"kb,attr"`
						Kxe  string `xml:"kxe,attr"`
					} `xml:"DesiredVirtualProcessors"`
					MaximumProcessingUnits struct {
						Text string `xml:",chardata"`
						Kxe  string `xml:"kxe,attr"`
						Kb   string `xml:"kb,attr"`
					} `xml:"MaximumProcessingUnits"`
					MaximumVirtualProcessors struct {
						Text string `xml:",chardata"`
						Kb   string `xml:"kb,attr"`
						Kxe  string `xml:"kxe,attr"`
					} `xml:"MaximumVirtualProcessors"`
					MinimumProcessingUnits struct {
						Text string `xml:",chardata"`
						Kxe  string `xml:"kxe,attr"`
						Kb   string `xml:"kb,attr"`
					} `xml:"MinimumProcessingUnits"`
					MinimumVirtualProcessors struct {
						Text string `xml:",chardata"`
						Kb   string `xml:"kb,attr"`
						Kxe  string `xml:"kxe,attr"`
					} `xml:"MinimumVirtualProcessors"`
					SharedProcessorPoolID struct {
						Text string `xml:",chardata"`
						Kb   string `xml:"kb,attr"`
						Kxe  string `xml:"kxe,attr"`
					} `xml:"SharedProcessorPoolID"`
					UncappedWeight struct {
						Text string `xml:",chardata"`
						Kxe  string `xml:"kxe,attr"`
						Kb   string `xml:"kb,attr"`
					} `xml:"UncappedWeight"`
				} `xml:"SharedProcessorConfiguration"`
				SharingMode struct {
					Text string `xml:",chardata"`
					Kb   string `xml:"kb,attr"`
					Kxe  string `xml:"kxe,attr"`
				} `xml:"SharingMode"`
				CurrentHasDedicatedProcessors struct {
					Text string `xml:",chardata"`
					Kxe  string `xml:"kxe,attr"`
					Kb   string `xml:"kb,attr"`
				} `xml:"CurrentHasDedicatedProcessors"`
				CurrentSharingMode struct {
					Text string `xml:",chardata"`
					Kxe  string `xml:"kxe,attr"`
					Kb   string `xml:"kb,attr"`
				} `xml:"CurrentSharingMode"`
				RuntimeHasDedicatedProcessors struct {
					Text string `xml:",chardata"`
					Kxe  string `xml:"kxe,attr"`
					Kb   string `xml:"kb,attr"`
				} `xml:"RuntimeHasDedicatedProcessors"`
				CurrentSharedProcessorConfiguration struct {
					Text          string `xml:",chardata"`
					Kb            string `xml:"kb,attr"`
					Kxe           string `xml:"kxe,attr"`
					SchemaVersion string `xml:"schemaVersion,attr"`
					Metadata      struct {
						Text string `xml:",chardata"`
						Atom string `xml:"Atom"`
					} `xml:"Metadata"`
					AllocatedVirtualProcessors struct {
						Text string `xml:",chardata"`
						Kb   string `xml:"kb,attr"`
						Kxe  string `xml:"kxe,attr"`
					} `xml:"AllocatedVirtualProcessors"`
					CurrentMaximumProcessingUnits struct {
						Text string `xml:",chardata"`
						Kxe  string `xml:"kxe,attr"`
						Kb   string `xml:"kb,attr"`
					} `xml:"CurrentMaximumProcessingUnits"`
					CurrentMinimumProcessingUnits struct {
						Text string `xml:",chardata"`
						Kb   string `xml:"kb,attr"`
						Kxe  string `xml:"kxe,attr"`
					} `xml:"CurrentMinimumProcessingUnits"`
					CurrentProcessingUnits struct {
						Text string `xml:",chardata"`
						Kb   string `xml:"kb,attr"`
						Kxe  string `xml:"kxe,attr"`
					} `xml:"CurrentProcessingUnits"`
					CurrentSharedProcessorPoolID struct {
						Text string `xml:",chardata"`
						Kxe  string `xml:"kxe,attr"`
						Kb   string `xml:"kb,attr"`
					} `xml:"CurrentSharedProcessorPoolID"`
					CurrentUncappedWeight struct {
						Text string `xml:",chardata"`
						Kxe  string `xml:"kxe,attr"`
						Kb   string `xml:"kb,attr"`
					} `xml:"CurrentUncappedWeight"`
					CurrentMinimumVirtualProcessors struct {
						Text string `xml:",chardata"`
						Kxe  string `xml:"kxe,attr"`
						Kb   string `xml:"kb,attr"`
					} `xml:"CurrentMinimumVirtualProcessors"`
					CurrentMaximumVirtualProcessors struct {
						Text string `xml:",chardata"`
						Kb   string `xml:"kb,attr"`
						Kxe  string `xml:"kxe,attr"`
					} `xml:"CurrentMaximumVirtualProcessors"`
					RuntimeProcessingUnits struct {
						Text string `xml:",chardata"`
						Kxe  string `xml:"kxe,attr"`
						Kb   string `xml:"kb,attr"`
					} `xml:"RuntimeProcessingUnits"`
					RuntimeUncappedWeight struct {
						Text string `xml:",chardata"`
						Kxe  string `xml:"kxe,attr"`
						Kb   string `xml:"kb,attr"`
					} `xml:"RuntimeUncappedWeight"`
				} `xml:"CurrentSharedProcessorConfiguration"`
			} `xml:"PartitionProcessorConfiguration"`
			PartitionProfiles struct {
				Text string `xml:",chardata"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
				Link struct {
					Text string `xml:",chardata"`
					Href string `xml:"href,attr"`
					Rel  string `xml:"rel,attr"`
				} `xml:"link"`
			} `xml:"PartitionProfiles"`
			PartitionState struct {
				Text string `xml:",chardata"`
				Kxe  string `xml:"kxe,attr"`
				Kb   string `xml:"kb,attr"`
			} `xml:"PartitionState"`
			PartitionType struct {
				Text string `xml:",chardata"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"PartitionType"`
			PartitionUUID struct {
				Text string `xml:",chardata"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"PartitionUUID"`
			PendingProcessorCompatibilityMode struct {
				Text string `xml:",chardata"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"PendingProcessorCompatibilityMode"`
			ProgressPartitionDataRemaining struct {
				Text string `xml:",chardata"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"ProgressPartitionDataRemaining"`
			ProgressPartitionDataTotal struct {
				Text string `xml:",chardata"`
				Kxe  string `xml:"kxe,attr"`
				Kb   string `xml:"kb,attr"`
			} `xml:"ProgressPartitionDataTotal"`
			ProgressState struct {
				Text string `xml:",chardata"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"ProgressState"`
			ResourceMonitoringControlState struct {
				Text string `xml:",chardata"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"ResourceMonitoringControlState"`
			AssociatedManagedSystem struct {
				Text string `xml:",chardata"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
				Href string `xml:"href,attr"`
				Rel  string `xml:"rel,attr"`
			} `xml:"AssociatedManagedSystem"`
			ClientNetworkAdapters struct {
				Text string `xml:",chardata"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
				Link struct {
					Text string `xml:",chardata"`
					Href string `xml:"href,attr"`
					Rel  string `xml:"rel,attr"`
				} `xml:"link"`
			} `xml:"ClientNetworkAdapters"`
			HostEthernetAdapterLogicalPorts struct {
				Text          string `xml:",chardata"`
				Kxe           string `xml:"kxe,attr"`
				Kb            string `xml:"kb,attr"`
				SchemaVersion string `xml:"schemaVersion,attr"`
				Metadata      struct {
					Text string `xml:",chardata"`
					Atom string `xml:"Atom"`
				} `xml:"Metadata"`
			} `xml:"HostEthernetAdapterLogicalPorts"`
			MACAddressPrefix struct {
				Text string `xml:",chardata"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"MACAddressPrefix"`
			IsServicePartition struct {
				Text string `xml:",chardata"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"IsServicePartition"`
			PowerVMManagementCapable struct {
				Text string `xml:",chardata"`
				Ksv  string `xml:"ksv,attr"`
				Kxe  string `xml:"kxe,attr"`
				Kb   string `xml:"kb,attr"`
			} `xml:"PowerVMManagementCapable"`
			ReferenceCode struct {
				Text string `xml:",chardata"`
				Kxe  string `xml:"kxe,attr"`
				Kb   string `xml:"kb,attr"`
			} `xml:"ReferenceCode"`
			AssignAllResources struct {
				Text string `xml:",chardata"`
				Ksv  string `xml:"ksv,attr"`
				Kxe  string `xml:"kxe,attr"`
				Kb   string `xml:"kb,attr"`
			} `xml:"AssignAllResources"`
			HardwareAcceleratorQoS struct {
				Text          string `xml:",chardata"`
				Ksv           string `xml:"ksv,attr"`
				Kxe           string `xml:"kxe,attr"`
				Kb            string `xml:"kb,attr"`
				SchemaVersion string `xml:"schemaVersion,attr"`
				Metadata      struct {
					Text string `xml:",chardata"`
					Atom string `xml:"Atom"`
				} `xml:"Metadata"`
			} `xml:"HardwareAcceleratorQoS"`
			LastActivatedProfile struct {
				Text string `xml:",chardata"`
				Ksv  string `xml:"ksv,attr"`
				Kxe  string `xml:"kxe,attr"`
				Kb   string `xml:"kb,attr"`
			} `xml:"LastActivatedProfile"`
			HasPhysicalIO struct {
				Text string `xml:",chardata"`
				Ksv  string `xml:"ksv,attr"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"HasPhysicalIO"`
			OperatingSystemType struct {
				Text string `xml:",chardata"`
				Ksv  string `xml:"ksv,attr"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"OperatingSystemType"`
			BootMode struct {
				Text string `xml:",chardata"`
				Ksv  string `xml:"ksv,attr"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"BootMode"`
			PowerOnWithHypervisor struct {
				Text string `xml:",chardata"`
				Ksv  string `xml:"ksv,attr"`
				Kxe  string `xml:"kxe,attr"`
				Kb   string `xml:"kb,attr"`
			} `xml:"PowerOnWithHypervisor"`
			MigrationStorageViosDataStatus struct {
				Text string `xml:",chardata"`
				Ksv  string `xml:"ksv,attr"`
				Kxe  string `xml:"kxe,attr"`
				Kb   string `xml:"kb,attr"`
			} `xml:"MigrationStorageViosDataStatus"`
			MigrationStorageViosDataTimestamp struct {
				Text string `xml:",chardata"`
				Ksv  string `xml:"ksv,attr"`
				Kxe  string `xml:"kxe,attr"`
				Kb   string `xml:"kb,attr"`
			} `xml:"MigrationStorageViosDataTimestamp"`
			RemoteRestartCapable struct {
				Text string `xml:",chardata"`
				Kxe  string `xml:"kxe,attr"`
				Kb   string `xml:"kb,attr"`
			} `xml:"RemoteRestartCapable"`
			SimplifiedRemoteRestartCapable struct {
				Text string `xml:",chardata"`
				Ksv  string `xml:"ksv,attr"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"SimplifiedRemoteRestartCapable"`
			HasDedicatedProcessorsForMigration struct {
				Text string `xml:",chardata"`
				Kxe  string `xml:"kxe,attr"`
				Kb   string `xml:"kb,attr"`
			} `xml:"HasDedicatedProcessorsForMigration"`
			SuspendCapable struct {
				Text string `xml:",chardata"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"SuspendCapable"`
			MigrationDisable struct {
				Text string `xml:",chardata"`
				Ksv  string `xml:"ksv,attr"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"MigrationDisable"`
			MigrationState struct {
				Text string `xml:",chardata"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"MigrationState"`
			RemoteRestartState struct {
				Text string `xml:",chardata"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
			} `xml:"RemoteRestartState"`
			VirtualFibreChannelClientAdapters struct {
				Text string `xml:",chardata"`
				Kb   string `xml:"kb,attr"`
				Kxe  string `xml:"kxe,attr"`
				Link []struct {
					Text string `xml:",chardata"`
					Href string `xml:"href,attr"`
					Rel  string `xml:"rel,attr"`
				} `xml:"link"`
			} `xml:"VirtualFibreChannelClientAdapters"`
			VirtualSCSIClientAdapters struct {
				Text string `xml:",chardata"`
				Kxe  string `xml:"kxe,attr"`
				Kb   string `xml:"kb,attr"`
				Link struct {
					Text string `xml:",chardata"`
					Href string `xml:"href,attr"`
					Rel  string `xml:"rel,attr"`
				} `xml:"link"`
			} `xml:"VirtualSCSIClientAdapters"`
			BootListInformation struct {
				Text          string `xml:",chardata"`
				Ksv           string `xml:"ksv,attr"`
				Kxe           string `xml:"kxe,attr"`
				Kb            string `xml:"kb,attr"`
				SchemaVersion string `xml:"schemaVersion,attr"`
				Metadata      struct {
					Text string `xml:",chardata"`
					Atom string `xml:"Atom"`
				} `xml:"Metadata"`
				PendingBootString struct {
					Text  string `xml:",chardata"`
					Group string `xml:"group,attr"`
					Ksv   string `xml:"ksv,attr"`
					Kxe   string `xml:"kxe,attr"`
					Kb    string `xml:"kb,attr"`
				} `xml:"PendingBootString"`
				BootDeviceList struct {
					Text  string `xml:",chardata"`
					Group string `xml:"group,attr"`
					Ksv   string `xml:"ksv,attr"`
					Kb    string `xml:"kb,attr"`
					Kxe   string `xml:"kxe,attr"`
				} `xml:"BootDeviceList"`
				ShadowBootDeviceList struct {
					Text  string `xml:",chardata"`
					Group string `xml:"group,attr"`
					Ksv   string `xml:"ksv,attr"`
					Kb    string `xml:"kb,attr"`
					Kxe   string `xml:"kxe,attr"`
				} `xml:"ShadowBootDeviceList"`
				LastBootedDeviceString struct {
					Text  string `xml:",chardata"`
					Group string `xml:"group,attr"`
					Ksv   string `xml:"ksv,attr"`
					Kb    string `xml:"kb,attr"`
					Kxe   string `xml:"kxe,attr"`
				} `xml:"LastBootedDeviceString"`
			} `xml:"BootListInformation"`
		} `xml:"LogicalPartition"`
	} `xml:"content"`
}

func (r *lpar) Name() string { return "lpar" }

func (r *lpar) login(l logger.Logger) error {
	lr := &LogonRequest{}
	lr.SchemaVersion = "V1_0"
	lr.Xmlns = "http://www.ibm.com/xmlns/systems/power/firmware/web/mc/2012_10/"
	lr.UserID = r.username
	lr.Password = r.password
	s, err := xml.Marshal(lr)
	if err != nil {
		return err
	}
	data := bytes.NewReader(s)
	url := fmt.Sprintf("%s/rest/api/web/Logon", r.url)

	// curl -k -c cookies.txt -i -X PUT
	//  -H "Content-Type: application/vnd.ibm.powervm.web+xml; type=LogonRequest"
	//  -H "Accept: application/vnd.ibm.powervm.web+xml; type=LogonResponse"
	//  -H "X-Audit-Memento: hmc_test"
	// -d @login.xml https://129.40.108.10:12443/rest/api/web/Logon

	req, err := http.NewRequest("PUT", url, data)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=LogonRequest")
	req.Header.Set("Accept", "application/vnd.ibm.powervm.web+xml; type=LogonResponse")
	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("Request failed: %v", err)
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	if resp.StatusCode >= 400 {
		rdata, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("login return %d: %s", resp.StatusCode, string(rdata))
	}

	rdata, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Failed to read body of login: %v", err)
	}

	lresp := &LogonResponse{}
	if lerr := xml.Unmarshal(rdata, &lresp); lerr != nil {
		return fmt.Errorf("failed to parse login response: %s", lerr)
	}
	// Should have auto filled the cookie jar
	return nil
}

func (r *lpar) logout() {
	// Currently, don't do anything.
}

func (r *lpar) poweron(l logger.Logger, lparId string) error {
	lr := &JobRequest{}
	lr.SchemaVersion = "V1_1_0"
	lr.Xmlns = "http://www.ibm.com/xmlns/systems/power/firmware/web/mc/2012_10/"
	lr.RequestedOperation.SchemaVersion = "V1_0"
	lr.RequestedOperation.Kb = "CUR"
	lr.RequestedOperation.Kxe = "false"
	lr.RequestedOperation.OperationName.Text = "PowerOn"
	lr.RequestedOperation.OperationName.Kb = "ROR"
	lr.RequestedOperation.OperationName.Kxe = "false"
	lr.RequestedOperation.GroupName.Text = "LogicalPartition"
	lr.RequestedOperation.GroupName.Kb = "ROR"
	lr.RequestedOperation.GroupName.Kxe = "false"
	lr.RequestedOperation.ProgressType.Text = "DISCRETE"
	lr.RequestedOperation.ProgressType.Kb = "ROR"
	lr.RequestedOperation.ProgressType.Kxe = "false"
	jp1 := JobParameter{}
	jp1.SchemaVersion = "V1_0"
	jp1.ParameterName.Text = "novsi"
	jp1.ParameterName.Kxe = "false"
	jp1.ParameterName.Kb = "ROR"
	jp1.ParameterValue.Text = "true"
	jp1.ParameterValue.Kxe = "false"
	jp1.ParameterValue.Kb = "CUR"
	jp2 := JobParameter{}
	jp2.SchemaVersion = "V1_0"
	jp2.ParameterName.Text = "force"
	jp2.ParameterName.Kxe = "false"
	jp2.ParameterName.Kb = "ROR"
	jp2.ParameterValue.Text = "false"
	jp2.ParameterValue.Kxe = "false"
	jp2.ParameterValue.Kb = "CUR"
	jp3 := JobParameter{}
	jp3.SchemaVersion = "V1_0"
	jp3.ParameterName.Text = "bootmode"
	jp3.ParameterName.Kxe = "false"
	jp3.ParameterName.Kb = "ROR"
	jp3.ParameterValue.Text = "norm"
	jp3.ParameterValue.Kxe = "false"
	jp3.ParameterValue.Kb = "CUR"
	lr.JobParameters.SchemaVersion = "V1_0"
	lr.JobParameters.Kxe = "false"
	lr.JobParameters.Kb = "CUR"
	lr.JobParameters.JobParameter = []JobParameter{
		jp1,
		jp2,
		jp3,
	}

	s, err := xml.Marshal(lr)
	if err != nil {
		return err
	}
	data := bytes.NewReader(s)
	url := fmt.Sprintf("%s/rest/api/uom/LogicalPartition/%s/do/PowerOn", r.url, lparId)

	// curl -k -c cookies.txt -b cookies.txt -i
	//  -H "Accept: application/atom+xml; charset=UTF-8"
	//  -H "Content-Type: application/vnd.ibm.powervm.web+xml; type=JobRequest"
	// https://129.40.108.10:12443/rest/api/uom/LogicalPartition/48EB2A9F-6028-41C6-8D2F-6A539361BB29/do/PowerOn -X PUT -d @power-on.xml

	req, err := http.NewRequest("PUT", url, data)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=JobRequest")
	req.Header.Set("Accept", "application/atom+xml; charset=UTF-8")
	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("Request poweron failed: %v", err)
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	if resp.StatusCode >= 400 {
		rdata, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("poweron return %d: %s", resp.StatusCode, string(rdata))
	}

	rdata, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Failed to read body of poweron: %v", err)
	}

	lresp := &JobEntry{}
	if lerr := xml.Unmarshal(rdata, &lresp); lerr != nil {
		return fmt.Errorf("failed to parse poweron response: %s", lerr)
	}
	// Should have auto filled the cookie jar
	return nil
}

func (r *lpar) poweroff(l logger.Logger, lparId, restart string) error {
	lr := &JobRequest{}
	lr.SchemaVersion = "V1_0"
	lr.Xmlns = "http://www.ibm.com/xmlns/systems/power/firmware/web/mc/2012_10/"
	lr.RequestedOperation.SchemaVersion = "V1_0"
	lr.RequestedOperation.Kb = "CUR"
	lr.RequestedOperation.Kxe = "false"
	lr.RequestedOperation.OperationName.Text = "PowerOff"
	lr.RequestedOperation.OperationName.Kb = "ROR"
	lr.RequestedOperation.OperationName.Kxe = "false"
	lr.RequestedOperation.GroupName.Text = "LogicalPartition"
	lr.RequestedOperation.GroupName.Kb = "ROR"
	lr.RequestedOperation.GroupName.Kxe = "false"
	lr.RequestedOperation.ProgressType.Text = "DISCRETE"
	lr.RequestedOperation.ProgressType.Kb = "ROR"
	lr.RequestedOperation.ProgressType.Kxe = "false"
	jp1 := JobParameter{}
	jp1.SchemaVersion = "V1_0"
	jp1.ParameterName.Text = "immediate"
	jp1.ParameterName.Kxe = "false"
	jp1.ParameterName.Kb = "ROR"
	jp1.ParameterValue.Text = "true"
	jp1.ParameterValue.Kxe = "false"
	jp1.ParameterValue.Kb = "CUR"
	jp2 := JobParameter{}
	jp2.SchemaVersion = "V1_0"
	jp2.ParameterName.Text = "restart"
	jp2.ParameterName.Kxe = "false"
	jp2.ParameterName.Kb = "ROR"
	jp2.ParameterValue.Text = restart
	jp2.ParameterValue.Kxe = "false"
	jp2.ParameterValue.Kb = "CUR"
	jp3 := JobParameter{}
	jp3.SchemaVersion = "V1_0"
	jp3.ParameterName.Text = "operation"
	jp3.ParameterName.Kxe = "false"
	jp3.ParameterName.Kb = "ROR"
	jp3.ParameterValue.Text = "shutdown"
	jp3.ParameterValue.Kxe = "false"
	jp3.ParameterValue.Kb = "CUR"
	lr.JobParameters.SchemaVersion = "V1_0"
	lr.JobParameters.Kxe = "false"
	lr.JobParameters.Kb = "CUR"
	lr.JobParameters.JobParameter = []JobParameter{
		jp1,
		jp2,
		jp3,
	}

	s, err := xml.Marshal(lr)
	if err != nil {
		return err
	}
	data := bytes.NewReader(s)
	url := fmt.Sprintf("%s/rest/api/uom/LogicalPartition/%s/do/PowerOff", r.url, lparId)

	// curl -k -c cookies.txt -b cookies.txt -i
	//  -H "Accept: application/atom+xml; charset=UTF-8"
	//  -H "Content-Type: application/vnd.ibm.powervm.web+xml; type=JobRequest"
	// https://129.40.108.10:12443/rest/api/uom/LogicalPartition/48EB2A9F-6028-41C6-8D2F-6A539361BB29/do/PowerOff -X PUT -d @power-off.xml

	req, err := http.NewRequest("PUT", url, data)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/vnd.ibm.powervm.web+xml; type=JobRequest")
	req.Header.Set("Accept", "application/atom+xml; charset=UTF-8")
	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("Request poweroff failed: %v", err)
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	if resp.StatusCode >= 400 {
		rdata, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("poweroff return %d: %s", resp.StatusCode, string(rdata))
	}

	rdata, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Failed to read body of poweroff: %v", err)
	}

	lresp := &JobEntry{}
	if lerr := xml.Unmarshal(rdata, &lresp); lerr != nil {
		return fmt.Errorf("failed to parse poweroff response: %s", lerr)
	}
	// Should have auto filled the cookie jar
	return nil
}

func (r *lpar) get(l logger.Logger, lparId string) (*GetEntry, error) {
	url := fmt.Sprintf("%s/rest/api/uom/LogicalPartition/%s", r.url, lparId)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/atom+xml; charset=UTF-8")
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to get %s: %v", lparId, err)
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	if resp.StatusCode >= 400 {
		rdata, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("get return %d: %s", resp.StatusCode, string(rdata))
	}

	rdata, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to read body of get: %v", err)
	}

	lresp := &GetEntry{}
	if lerr := xml.Unmarshal(rdata, &lresp); lerr != nil {
		return nil, fmt.Errorf("failed to parse get response: %s", lerr)
	}

	return lresp, nil
}

func (r *lpar) Probe(l logger.Logger, address string, port int, username, password string) bool {
	r.username = username
	r.password = password
	r.url = fmt.Sprintf("https://%s", net.JoinHostPort(address, strconv.Itoa(port)))

	defaultTransport := http.DefaultTransport.(*http.Transport)
	transport := &http.Transport{
		Proxy:                 defaultTransport.Proxy,
		DialContext:           defaultTransport.DialContext,
		MaxIdleConns:          defaultTransport.MaxIdleConns,
		IdleConnTimeout:       defaultTransport.IdleConnTimeout,
		ExpectContinueTimeout: defaultTransport.ExpectContinueTimeout,
		TLSHandshakeTimeout:   time.Duration(3) * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		l.Errorf("Failed to initialize the cookie jar: %v", err)
		return false
	}

	r.client = &http.Client{Transport: transport, Jar: jar}
	if err := r.login(l); err != nil {
		l.Errorf("Failed to log into the HMC: %v", err)
		return false
	}
	return true
}

func (r *lpar) Action(l logger.Logger, ma *models.Action) (supported bool, res interface{}, err *models.Error) {
	// Close session when done
	defer r.logout()

	lparId, ok := ma.Params["ipmi/lpar-id"].(string)
	if !ok {
		supported = true
		res = struct{}{}
		err = &models.Error{
			Model: "plugin",
			Key:   "ipmi",
			Type:  "rpc",
			Code:  400,
		}
		err.Errorf("LPAR id missing: specify ipmi/lpar-id")
		return
	}

	switch ma.Command {
	case "powerstatus":
		supported = true
		res = struct{}{}
		ge, gerr := r.get(l, lparId)
		if gerr != nil {
			err = &models.Error{
				Model: "plugin",
				Key:   "ipmi",
				Type:  "rpc",
				Code:  400,
			}
			err.Errorf("LPAR get failed: %v", gerr)
			return
		}
		res = ge.Content.LogicalPartition.PartitionState.Text
		err = nil
	case "poweron":
		perr := r.poweron(l, lparId)
		if perr != nil {
			supported = true
			res = struct{}{}
			err = &models.Error{
				Model: "plugin",
				Key:   "ipmi",
				Type:  "rpc",
				Code:  400,
			}
			err.Errorf("LPAR poweron failed: %v", perr)
			return
		}
		supported = true
		res = "Success"
		err = nil
	case "poweroff":
		perr := r.poweroff(l, lparId, "false")
		if perr != nil {
			supported = true
			res = struct{}{}
			err = &models.Error{
				Model: "plugin",
				Key:   "ipmi",
				Type:  "rpc",
				Code:  400,
			}
			err.Errorf("LPAR poweroff failed: %v", perr)
			return
		}
		supported = true
		res = "Success"
		err = nil
	case "powercycle":
		perr := r.poweroff(l, lparId, "true")
		if perr != nil {
			supported = true
			res = struct{}{}
			err = &models.Error{
				Model: "plugin",
				Key:   "ipmi",
				Type:  "rpc",
				Code:  400,
			}
			err.Errorf("LPAR powercycle(poweroff) failed: %v", perr)
			return
		}
		supported = true
		res = "Success"
		err = nil
	case "nextbootpxe", "nextbootdisk", "forcebootpxe", "forcebootdisk":
		supported = true
		res = "Success"
		err = nil
	}
	return
}
